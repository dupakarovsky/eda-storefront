package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/dupakarovsky/eda-storefront/foundation/shutdown"
)

func main() {

	logger, logFile, err := logging.New("tmp-smoketest.log", "debug", "info")
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	srv := &http.Server{Addr: ":8081"}

	if err := shutdown.Run(logger, srv, 5*time.Second); err != nil {
		fmt.Println("shutdown.Run returned an error:", err)
	}

	fmt.Println("main exited")
}
