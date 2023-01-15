package direct

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/rmb-sdk-go/direct/types"
	"github.com/threefoldtech/substrate-client"
)

func TestSignature(t *testing.T) {
	manager := substrate.NewManager("wss://tfchain.dev.grid.tf/ws")
	sub, err := manager.Substrate()
	if err != nil {
		t.Fatalf("could not initialize substrate connection: %s", err)
	}
	defer sub.Close()

	identity, err := substrate.NewIdentityFromSr25519Phrase("")
	if err != nil {
		t.Fatalf("could not init new identity: %s", err)
	}

	env := types.Envelope{
		Uid:         uuid.NewString(),
		Timestamp:   uint64(time.Now().Unix()),
		Expiration:  10000,
		Destination: &types.Address{Twin: 10},
	}

	env.Message = &types.Envelope_Request{
		Request: &types.Request{
			Command: "cmd",
			Data:    []byte("data"),
		},
	}
	t.Run("valid signature", func(t *testing.T) {
		env.Source = &types.Address{
			Twin: 49,
		}

		toSign, err := Challenge(&env)
		assert.NoError(t, err)

		env.Signature, err = Sign(identity, toSign)
		assert.NoError(t, err)

		err = VerifySignature(sub, &env)
		assert.NoError(t, err)
	})

	t.Run("invalid source", func(t *testing.T) {
		env.Source = &types.Address{
			Twin: 2,
		}

		toSign, err := Challenge(&env)
		assert.NoError(t, err)

		env.Signature, err = Sign(identity, toSign)
		assert.NoError(t, err)

		err = VerifySignature(sub, &env)
		assert.Error(t, err)
	})

	t.Run("invalid signature", func(t *testing.T) {
		env.Source = &types.Address{
			Twin: 49,
		}

		env.Signature = []byte("s13p49fnaskdjnv")

		err = VerifySignature(sub, &env)
		assert.Error(t, err)
	})
}
