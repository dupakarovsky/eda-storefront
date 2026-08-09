package main

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/foundation/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("error loading config: ", err)
		return
	}

	fmt.Printf("%+v\n", cfg)
	fmt.Println("addr:", cfg.Server.Addr())
}
