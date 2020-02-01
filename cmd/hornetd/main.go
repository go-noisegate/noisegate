package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/ks888/hornet/common/log"
	"github.com/ks888/hornet/server"
	"github.com/urfave/cli"
)

func main() {
	app := &cli.App{
		Name:      filepath.Base(os.Args[0]),
		ArgsUsage: "[server address (default: \"localhost:48059\")]",
		Action: func(c *cli.Context) error {
			// TODO: make sure it's accessible from docker containers.
			addr := "localhost:48059" // bees
			if c.NArg() > 0 {
				addr = c.Args().First()
			}

			log.EnableDebugLog(c.Bool("debug"))

			opt := workerOptions{
				addrFromContainer: c.String("address-from-container"),
				numWorkers:        c.Int("num-workers"),
				workerPath:        c.String("worker-path"),
				image:             c.String("image"),
			}
			return runServer(addr, opt)
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
			&cli.IntFlag{
				Name:  "num-workers",
				Usage: "the number of workers",
				Value: 4,
			},
			&cli.StringFlag{
				Name:  "worker-path",
				Usage: "path to the `hornet-worker` binary. If empty, search the PATH directories",
				Value: "",
			},
			&cli.StringFlag{
				Name:  "image",
				Usage: "the docker image the workers use",
				Value: "alpine:3.11",
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
	addrFromContainer string
	numWorkers        int
	workerPath        string
	image             string
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

	workerManager := &server.WorkerManager{ServerAddress: opt.addrFromContainer, WorkerBinPath: opt.workerPath}
	// TODO: remove workers if the process exits here (or always remove old workers here anyway?)
	log.Printf("start %d workers\n", opt.numWorkers)
	for i := 0; i < opt.numWorkers; i++ {
		if err := workerManager.AddWorker("", opt.image); err != nil {
			return fmt.Errorf("failed to add the worker #%d: %w", i, err)
		}
	}

	server := server.NewHornetServer(addr, workerManager)
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
