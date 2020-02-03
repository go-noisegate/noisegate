package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ks888/hornet/common/log"
	"github.com/ks888/hornet/worker"
	"github.com/urfave/cli"
)

func main() {
	app := &cli.App{
		Name:      filepath.Base(os.Args[0]),
		ArgsUsage: "[worker group] [worker id]",
		Action: func(c *cli.Context) error {
			if c.NArg() < 2 {
				return errors.New("worker group and/or worker id is not specified")
			}

			log.EnableDebugLog(c.Bool("debug"))

			groupName := c.Args().Get(0)
			workerID, err := strconv.Atoi(c.Args().Get(1))
			if err != nil {
				return err
			}
			w := worker.Executor{GroupName: groupName, ID: workerID, Addr: c.String("addr")}
			return w.Run(c.Context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Usage: "hornet server's address",
				Value: "host.docker.internal:48059",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "print the debug logs",
				Value: false,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
