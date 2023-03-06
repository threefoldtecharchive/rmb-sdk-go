package direct

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/substrate-client"
)

type Connection struct {
	con             *websocket.Conn
	m               sync.Mutex
	startReading    chan bool
	twinID          uint32
	session         string
	identity        *substrate.Identity
	relayDomain     string
	incomingMessage chan incomingMessage
	isReconnecting  bool
	reconnectingM   sync.RWMutex
}

type incomingMessage struct {
	messageType int
	data        []byte
}

func NewConnection(identity substrate.Identity, relayDomain string, session string, twinID uint32) (*Connection, error) {
	token, err := NewJWT(identity, twinID, session, 60)
	if err != nil {
		return nil, errors.Wrap(err, "could not create new jwt")
	}

	relayUrl := fmt.Sprintf("%s?%s", relayDomain, token)
	con, resp, err := websocket.DefaultDialer.Dial(relayUrl, nil)
	if err != nil {
		var body []byte
		var status string
		if resp != nil {
			status = resp.Status
			body, _ = io.ReadAll(resp.Body)
		}
		return nil, errors.Wrapf(err, "failed to connect (%s): %s", status, string(body))
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("invalid response %s", resp.Status)
	}

	con.SetPongHandler(func(appData string) error {
		_ = con.SetReadDeadline(time.Now().Add(time.Minute))
		return nil
	})

	c := Connection{
		con:             con,
		twinID:          twinID,
		identity:        &identity,
		relayDomain:     relayDomain,
		session:         session,
		m:               sync.Mutex{},
		startReading:    make(chan bool),
		incomingMessage: make(chan incomingMessage),
		isReconnecting:  false,
		reconnectingM:   sync.RWMutex{},
	}
	go c.listen(context.Background())
	go c.ping(context.Background())
	c.startReading <- true

	return &c, nil
}

func (c *Connection) listen(ctx context.Context) {
	// Read tries to read from the websocket connection
	// if connection is closed, a reconnect is initiated, then retry read
	// this should replace process
	// Read should handle incoming pongs, and if no pongs are received for one minute, a reconnect should be initiated
	//
	// returns only when context is canceled
	for {
		<-c.startReading
		for {
			typ, data, err := c.con.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("websocket error connection closed")
				c.tryReconnect(ctx)
				break
			}

			c.incomingMessage <- incomingMessage{
				messageType: typ,
				data:        data,
			}

		}
	}
}

func (c *Connection) tryReconnect(ctx context.Context) {
	c.reconnectingM.RLock()
	if !c.isReconnecting {
		go c.reconnect(ctx)
	}
	c.reconnectingM.RUnlock()
}

func (c *Connection) ping(ctx context.Context) {
	for {
		<-time.After(20 * time.Second)
		c.m.Lock()
		err := c.con.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Minute))
		if err != nil {
			log.Error().Err(err).Msg("could not send ping message")
			c.tryReconnect(ctx)
		}
		c.m.Unlock()
	}
}

func (c *Connection) Write(ctx context.Context, messageType int, bytes []byte) error {
	// Write tries to write to the websocket connection
	// if connection is closed, a reconnect is initiated, then retry write
	//
	// returns when write is successful or context is canceled
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := c.writeMessage(messageType, bytes)
			if err == nil {
				return nil
			}
			// TODO: only retry if this is a connection issue, else, return the error
			log.Error().Err(err).Msg("could not send message")
			c.tryReconnect(ctx)
		}
	}

}

func (c *Connection) writeMessage(messageType int, bytes []byte) error {
	c.m.Lock()
	defer c.m.Unlock()
	c.con.SetWriteDeadline(time.Now().Add(1 * time.Second))
	return c.con.WriteMessage(messageType, bytes)
}

func (c *Connection) reconnect(ctx context.Context) error {
	/*
		a reconnect should happen whenever:
			- no pongs are received for a minute
			- write failed with connection closed
			- read failed with connection closed
		how to reconnect?
			- generate new jwt
			- create new source
			- create new websocket connection
			- reassign websocket connection and source
	*/
	// close should stop reading, Read now should be waiting for a start signal
	c.reconnectingM.Lock()
	c.isReconnecting = true
	c.reconnectingM.Unlock()

	defer func() {
		c.reconnectingM.Lock()
		c.isReconnecting = false
		c.reconnectingM.Unlock()
	}()

	c.m.Lock()
	defer c.m.Unlock()

	c.con.Close()

	token, err := NewJWT(*c.identity, c.twinID, c.session, 60)
	if err != nil {
		return errors.Wrap(err, "could not create new jwt")
	}

	relayUrl := fmt.Sprintf("%s?%s", c.relayDomain, token)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		<-time.After(1 * time.Second)

		con, resp, err := websocket.DefaultDialer.Dial(relayUrl, nil)
		if err != nil {
			var body []byte
			var status string
			if resp != nil {
				status = resp.Status
				body, _ = io.ReadAll(resp.Body)
			}
			log.Error().Err(err).Msgf("failed to connect (%s): %s", status, string(body))
			continue
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			log.Error().Msgf("invalid response %s", resp.Status)
			continue
		}
		con.SetPongHandler(func(appData string) error {
			_ = con.SetReadDeadline(time.Now().Add(time.Minute))
			return nil
		})
		c.con = con
		break

	}

	c.startReading <- true
	return nil
}

/*
	direct client should be able to read and write message on relay websocket
	if websocket connection is closed during read or write operations, it should be reconnected
	reconnection should be abstracted from direct client
	direct client is always listening for incoming messages from relay
	direct client keeps pinging the relay to keep the connection alive
	direct client is always listening for a reconnect signal
	if reconnect is initiated:
		- read routine is stopped
		- no writes are permitted
		- a new connection the relay is established
		- a new read routine is spawned
*/
