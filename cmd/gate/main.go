package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/go-noisegate/noisegate/client"
	"github.com/go-noisegate/noisegate/common"
	"github.com/go-noisegate/noisegate/common/log"
	"github.com/urfave/cli/v2"
)

const testCommandUsage = "Run tests affected by recent changes"
const testCommandDesc = testCommandUsage + `.

   Args after '--' are passed to the 'go test' command.`
const hintCommandUsage = "Hint recent changes"
const hintCommandDesc = hintCommandUsage + `.`

func main() {
	app := &cli.App{
		Name:  filepath.Base(os.Args[0]),
		Usage: "CLI for Noise Gate",
		Commands: []*cli.Command{
			{
				Name:        "test",
				Usage:       testCommandUsage,
				Description: testCommandDesc,
				ArgsUsage:   "[directory path] -- [go test options]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the path is not specified")
					}

					log.EnableDebugLog(c.Bool("debug"))

					query := c.Args().First()
					options := client.TestOptions{ServerAddr: c.String("addr"), TestLogger: os.Stdout, Bypass: c.Bool("bypass")}
					if c.Args().Len() > 1 && c.Args().Get(1) == "--" {
						options.GoTestOptions = c.Args().Slice()[2:]
					}
					return client.TestAction(c.Context, query, options)
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "bypass",
						Usage: "run all tests regardless of recent changes",
					},
				},
			},
			{
				Name:        "hint",
				Usage:       hintCommandUsage,
				Description: hintCommandDesc,
				ArgsUsage:   "[filepath:#begin-end (e.g. sum.go:#1-2)]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the target file is not specified")
					}

					log.EnableDebugLog(c.Bool("debug"))

					filepath := c.Args().First()
					options := client.HintOptions{ServerAddr: c.String("addr")}
					return client.HintAction(c.Context, filepath, options)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Usage: "gated server's `address`",
				Value: "localhost:48059",
			},
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
