package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/ks888/hornet/server"
)

func main() {
	addr := "localhost:48059" // bees
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}

	testBinaryDir, err := ioutil.TempDir("", "hornet")
	if err != nil {
		log.Fatalf("failed to create the directory to store the test binary: %v", err)
	}

	log.Fatal(server.Run(addr, testBinaryDir))
}
