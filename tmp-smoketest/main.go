package main

import (
	"context"
	"fmt"
	"os"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

func main() {

	// create the connection and jetstream context
	nc, js, err := foundationnats.Connect(foundationnats.Config{
		URL:  "nats://localhost:6969",
		Name: "tmp-smoketest",
	})
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	defer nc.Close()

	fmt.Println("connected: ", nc.ConnectedUrl())
	fmt.Println("status:", nc.Status())
	fmt.Println("jetstream context ready", js != nil)

	ctx := context.Background()

	// defines the STORES stream with the subject "stores.>"
	stream, err := foundationnats.EnsureStream(ctx, js, "STORES", []string{"stores.>"})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("stream ready:", stream)

	// instantiate a new publisher
	pub := foundationnats.NewPublisher(js)

	// define a new subject
	subject := foundationnats.Subject("stores", "store", "created")

	// publish to the stream and receive an acknowledgment
	ack, err := pub.Publish(ctx, subject, []byte(`{"store_id":"abc123"`))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("published to: ", ack.Stream, "seq: ", ack.Sequence)
}
