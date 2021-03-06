package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/go-noisegate/noisegate/common"
	"github.com/go-noisegate/noisegate/common/log"
	"github.com/go-noisegate/noisegate/server"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:      filepath.Base(os.Args[0]),
		Usage:     "Server for Noise Gate",
		ArgsUsage: "[server address (default: \"localhost:48059\")]",
		Action: func(c *cli.Context) error {
			addr := "localhost:48059"
			if c.NArg() > 0 {
				addr = c.Args().First()
			}

			log.EnableDebugLog(c.Bool("debug"))

			return runServer(addr)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "print the debug logs",
				Value: false,
			},
		},
		HideHelpCommand: true,
		Version:         common.Version,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func runServer(addr string) error {
	server := server.NewServer(addr)
	shutdownDoneCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh

		log.Println("shut down")
		const timeout = 3 * time.Second
		ctx, _ := context.WithTimeout(context.Background(), timeout)
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		close(shutdownDoneCh)
	}()

	log.Printf("start the server at %s\n", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("failed to start or close the server: %w", err)
	}

	<-shutdownDoneCh
	return nil
}
