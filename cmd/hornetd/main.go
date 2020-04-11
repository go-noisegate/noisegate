package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/ks888/hornet/common/log"
	"github.com/ks888/hornet/server"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:      filepath.Base(os.Args[0]),
		ArgsUsage: "[server address (default: \"localhost:48059\")]",
		Action: func(c *cli.Context) error {
			addr := "localhost:48059" // bees
			if c.NArg() > 0 {
				addr = c.Args().First()
			}

			log.EnableDebugLog(c.Bool("debug"))

			numWorkers := c.Int("num-workers")
			if numWorkers < 1 {
				return errors.New("the number of workers must be positive")
			}
			opt := workerOptions{
				numWorkers: numWorkers,
			}
			return runServer(addr, opt)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "print the debug logs",
				Value: false,
			},
			&cli.IntFlag{
				Name:  "num-workers",
				Usage: "the number of workers",
				Value: 4,
			},
		},
		HideHelp: true, // to hide the `COMMANDS` section in the help message.
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type workerOptions struct {
	numWorkers int
}

func runServer(addr string, opt workerOptions) error {
	tmpDir := "/tmp/hornet"
	_ = os.Mkdir(tmpDir, os.ModePerm) // may exist already

	sharedDir, err := ioutil.TempDir(tmpDir, "hornet")
	if err != nil {
		return fmt.Errorf("failed to create the directory to store the test binary: %w", err)
	}
	defer os.RemoveAll(sharedDir)
	server.SetUpSharedDir(sharedDir)

	server := server.NewHornetServer(addr)
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
