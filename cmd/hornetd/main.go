package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/ks888/hornet/server"
)

func main() {
	addr := "localhost:48059" // bees
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}

	if err := runServer(addr); err != nil {
		log.Fatal(err)
	}
}

func runServer(addr string) error {
	sharedDir, err := ioutil.TempDir("", "hornet")
	if err != nil {
		log.Fatalf("failed to create the directory to store the test binary: %v", err)
	}
	defer os.RemoveAll(sharedDir)

	server := server.NewHornetServer(addr, sharedDir)
	shutdownDoneCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh

		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("failed to shutdown the server: %v", err)
		}
		close(shutdownDoneCh)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("failed to start or close the server: %w", err)
	}

	<-shutdownDoneCh
	return nil
}
