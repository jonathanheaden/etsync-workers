package main

import (
	"fmt"
  log "github.com/sirupsen/logrus"

)

func main() {
	config, err := LoadConfig(".")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	fmt.Println(config.MONGO_URI)
}
