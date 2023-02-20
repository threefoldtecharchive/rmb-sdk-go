package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/threefoldtech/rmb-sdk-go/direct"
	"github.com/threefoldtech/substrate-client"
)

func app() error {
	mnemonics := "<menmonics goes here>"
	identity, err := substrate.NewIdentityFromSr25519Phrase(mnemonics)
	if err != nil {
		return err
	}
	sk, err := direct.GenerateSecureKey(mnemonics)
	if err != nil {
		return errors.Wrapf(err, "could not generate secure key")
	}
	subManager := substrate.NewManager("wss://tfchain.dev.grid.tf/ws")
	sub, err := subManager.Substrate()
	if err != nil {
		return fmt.Errorf("failed to connect to substrate: %w", err)
	}
	defer sub.Close()
	twinDB := direct.NewTwinDB(sub)
	client, err := direct.NewClient(context.Background(), identity, "wss://457f-197-63-218-127.eu.ngrok.io", "test-client", twinDB, sk)
	if err != nil {
		return fmt.Errorf("failed to create direct client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const dst = 7 // <- replace this with the twin id of where the service is running
	// it's okay to run both the server and the client behind the same rmb-peer
	var output float64
	for i := 0; i < 20; i++ {
		if err := client.Call(ctx, dst, "calculator.add", []float64{output, float64(i)}, &output); err != nil {
			return err
		}
	}

	fmt.Printf("output: %f\n", output)

	return nil
}

func main() {
	if err := app(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
