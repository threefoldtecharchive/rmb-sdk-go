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

	var id uint32 = 7 //your twin id goes here
	client, err := direct.NewClient(context.Background(), identity, "ws://localhost:8080", id, "test-client")

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const dst = 7 // <- replace this with the twin id of where the service is running
	// it's okay to run both the server and the client behind the same rmb-peer
	var output float64
	fmt.Println("making call")
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
