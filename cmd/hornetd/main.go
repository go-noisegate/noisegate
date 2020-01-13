package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/ks888/hornet/common/log"
	"github.com/ks888/hornet/server"
	"github.com/urfave/cli"
)

func main() {
	app := &cli.App{
		Name:      filepath.Base(os.Args[0]),
		ArgsUsage: "[server address]",
		Action: func(c *cli.Context) error {
			// TODO: make sure it's accessible from docker containers.
			addr := "localhost:48059" // bees
			if c.NArg() > 0 {
				addr = c.Args().First()
			}

			log.EnableDebugLog(c.Bool("debug"))

			return runServer(addr, c.String("address-from-container"))
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "print the debug logs",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "address-from-container",
				Usage: "address to access hornetd server from container",
				Value: "host.docker.internal:48059",
			},
		},
		HideHelp: true, // to hide the `COMMANDS` section in the help message.
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func runServer(addr, addrFromContainer string) error {
	sharedDir, err := ioutil.TempDir("", "hornet")
	if err != nil {
		return fmt.Errorf("failed to create the directory to store the test binary: %w", err)
	}
	defer os.RemoveAll(sharedDir)

	manager := server.NewJobManager()
	server := server.NewHornetServer(addr, sharedDir, manager)
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
