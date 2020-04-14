package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ks888/noisegate/client"
	"github.com/ks888/noisegate/common/log"
	"github.com/urfave/cli/v2"
)

const testCommandUsage = "Run tests based on recent changes"
const testCommandDesc = testCommandUsage + `.

   The query of this command is also considered as the recent change.
   See the help of the 'gate hint' command for the usage of the query.

   Args after '--' are passed to the 'go test' command.`
const hintCommandUsage = "Hint recent changes"
const hintCommandDesc = hintCommandUsage + `.

   Use the query to specify the change.
   * The part of the file is changed: [filepath:#begin-end]    e.g. sum.go:#1-2
   * The entire file is changed:      [filepath]               e.g. sum.go
   * The entire package is changed:   [package directory path] e.g. ./pkg_dir`

func main() {
	app := &cli.App{
		Name: filepath.Base(os.Args[0]),
		Commands: []*cli.Command{
			{
				Name:        "test",
				Aliases:     []string{"t"},
				Usage:       testCommandUsage,
				Description: testCommandDesc,
				ArgsUsage:   "[query (e.g. sum.go:#1-2)] -- [go test options]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the file path is not specified")
					}

					log.EnableDebugLog(c.Bool("debug"))

					query := c.Args().First()
					options := client.TestOptions{ServerAddr: c.String("addr"), TestLogger: os.Stdout}
					if c.Args().Len() > 1 && c.Args().Get(1) == "--" {
						options.GoTestOptions = c.Args().Slice()[2:]
					}
					return client.TestAction(c.Context, query, options)
				},
			},
			{
				Name:        "hint",
				Usage:       hintCommandUsage,
				Description: hintCommandDesc,
				ArgsUsage:   "[query (e.g. sum.go:#1-2)]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the target file or directory path is not specified")
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
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
