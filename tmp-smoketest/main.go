package main

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/foundation/logging"
)

func main() {
	logger, file, err := logging.New("smoketest.log", "debug", "info")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	logger.Info("foundation import resolved across workspace boundary")
	fmt.Println("smoke test passed")
}
