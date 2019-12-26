package main

import (
	"log"
	"os"

	"github.com/ks888/hornet/server"
)

func main() {
	addr := "localhost:48059" // bees
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}

	log.Fatal(server.Run(addr))
}
