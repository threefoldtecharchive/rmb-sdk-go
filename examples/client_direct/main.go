package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/threefoldtech/rmb-sdk-go/direct"
	"github.com/threefoldtech/substrate-client"
)

func app() error {

	identity, err := substrate.NewIdentityFromEd25519Phrase("<menmonics goes here>")
	if err != nil {
		return err
	}
	subManager := substrate.NewManager("wss://tfchain.dev.grid.tf/ws")
	sub, err := subManager.Substrate()
	if err != nil {
		return err
	}
	defer sub.Close()
	twinDB := direct.NewTwinDB(sub)
	client, err := direct.NewClient(context.Background(), identity, "wss://relay.dev.grid.tf", "test-client", twinDB)

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const dst = 7 // <- replace this with the twin id of where the service is running
	// it's okay to run both the server and the client behind the same rmb-peer
	var output float64
	if err := client.Call(ctx, dst, "calculator.add", []float64{10, 20}, &output); err != nil {
		return err
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
