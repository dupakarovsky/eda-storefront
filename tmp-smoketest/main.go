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

	// create the stream
	stream, err := foundationnats.EnsureStream(ctx, js, "TEST_STREAM", []string{"test.>"})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// get stream info
	info, err := stream.Info(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("stream:", info.Config.Name, "subjects:", info.Config.Subjects)
	fmt.Println("sample subject:", foundationnats.Subjects("stores", "store", "created"))
}
