package main

import (
	"log"

	"github.com/ks888/hornet/server"
)

func main() {
	log.Fatal(server.Run("localhost:8080"))
}
