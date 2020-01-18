package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ks888/hornet/client"
	"github.com/ks888/hornet/common/log"
	"github.com/urfave/cli"
)

func main() {
	app := &cli.App{
		Name: filepath.Base(os.Args[0]),
		Commands: []*cli.Command{
			{
				Name:      "test",
				Aliases:   []string{"t"},
				Usage:     "Run a test",
				ArgsUsage: "[target filepath]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the target filepath is not specified")
					}

					log.EnableDebugLog(c.Bool("debug"))

					filepath := c.Args().First()
					options := client.TestOptions{ServerAddr: c.String("addr"), TestLogger: os.Stdout}
					return client.TestAction(c.Context, filepath, options)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Usage: "hornetd server's address",
				Value: "localhost:48059",
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
