package direct

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/rmb-sdk-go"
	"github.com/threefoldtech/rmb-sdk-go/direct/types"
	"github.com/threefoldtech/substrate-client"
	"google.golang.org/protobuf/proto"
)

type directClient struct {
	source    *types.Address
	signer    substrate.Identity
	con       *websocket.Conn
	responses map[string]chan *types.Envelope
	m         sync.Mutex
	twinDB    TwinDB
}

// id is the twin id that is associated with the given identity.
func NewClient(ctx context.Context, identity substrate.Identity, url string, session string, twinDB TwinDB) (rmb.Client, error) {
	id, err := twinDB.GetByPk(identity.PublicKey())
	if err != nil {
		return nil, err
	}

	token, err := NewJWT(identity, id, session, 60) // use 1 min token ttl
	if err != nil {
		return nil, errors.Wrap(err, "failed to build authentication token")
	}
	// wss://relay.dev.grid.tf/?<JWT>
	url = fmt.Sprintf("%s?%s", url, token)
	source := types.Address{
		Twin:       id,
		Connection: &session,
	}

	con, resp, err := websocket.DefaultDialer.Dial(url, nil)
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

	cl := &directClient{
		source:    &source,
		signer:    identity,
		con:       con,
		responses: make(map[string]chan *types.Envelope),
		twinDB:    twinDB,
	}

	go cl.process()
	return cl, nil
}

func (d *directClient) process() {
	defer d.con.Close()
	// todo: set error on connection here
	for {
		typ, msg, err := d.con.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("websocket error connection closed")
			return
		}
		if typ != websocket.BinaryMessage {
			continue
		}

		var env types.Envelope
		if err := proto.Unmarshal(msg, &env); err != nil {
			log.Error().Err(err).Msg("invalid message payload")
			return
		}

		d.router(&env)
	}
}

func (d *directClient) router(env *types.Envelope) {
	d.m.Lock()
	defer d.m.Unlock()

	ch, ok := d.responses[env.Uid]
	if !ok {
		return
	}

	select {
	case ch <- env:
	default:
		// client is not waiting anymore! just return then
	}
}

func (d *directClient) makeRequest(dest uint32, cmd string, data []byte, ttl uint64) (*types.Envelope, error) {
	schema := rmb.DefaultSchema

	env := types.Envelope{
		Uid:         uuid.NewString(),
		Timestamp:   uint64(time.Now().Unix()),
		Expiration:  ttl,
		Source:      d.source,
		Destination: &types.Address{Twin: dest},
		Schema:      &schema,
	}

	env.Message = &types.Envelope_Request{
		Request: &types.Request{
			Command: cmd,
		},
	}

	env.Payload = &types.Envelope_Plain{
		Plain: data,
	}

	toSign, err := Challenge(&env)
	if err != nil {
		return nil, err
	}

	env.Signature, err = Sign(d.signer, toSign)
	if err != nil {
		return nil, err
	}

	return &env, nil

}

func (d *directClient) Call(ctx context.Context, twin uint32, fn string, data interface{}, result interface{}) error {

	payload, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "failed to serialize request body")
	}

	var ttl uint64 = 5 * 60
	deadline, ok := ctx.Deadline()
	if ok {
		ttl = uint64(time.Until(deadline).Seconds())
	}

	request, err := d.makeRequest(twin, fn, payload, ttl)
	if err != nil {
		return errors.Wrap(err, "failed to build request")
	}

	ch := make(chan *types.Envelope)
	d.m.Lock()
	d.responses[request.Uid] = ch
	d.m.Unlock()

	bytes, err := proto.Marshal(request)
	if err != nil {
		return err
	}

	if err := d.con.WriteMessage(websocket.BinaryMessage, bytes); err != nil {
		return err
	}

	var response *types.Envelope
	select {
	case <-ctx.Done():
		return ctx.Err()
	case response = <-ch:
	}
	if response == nil {
		// shouldn't happen but just in case
		return fmt.Errorf("no response received")
	}

	errResp := response.GetError()
	if response.Source == nil {
		// an envelope received that has NO source twin
		// this is possible only if the relay returned an error
		// hence
		if errResp != nil {
			return fmt.Errorf(errResp.Message)
		}

		// otherwise that's a malformed message
		return fmt.Errorf("received an invalid envelope")
	}

	err = VerifySignature(d.twinDB, response)
	if err != nil {
		return errors.Wrap(err, "message signature verification failed")
	}

	if errResp != nil {
		// todo: include code also
		return fmt.Errorf(errResp.Message)
	}

	resp := response.GetResponse()
	if resp == nil {
		return fmt.Errorf("received a non response envelope")
	}

	if result == nil {
		return nil
	}

	if response.Schema == nil || *response.Schema != rmb.DefaultSchema {
		return fmt.Errorf("invalid schema received expected '%s'", rmb.DefaultSchema)
	}

	var output []byte
	switch payload := response.Payload.(type) {
	case *types.Envelope_Cipher:
		// TODO: implement handler for cipher data
		// we need then to decrypt the data
		return fmt.Errorf("(not implemented) encrypted payload is not supported yet")
	case *types.Envelope_Plain:
		output = payload.Plain
	}

	return json.Unmarshal(output, &result)
}
