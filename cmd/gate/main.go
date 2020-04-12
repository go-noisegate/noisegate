package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ks888/noisegate/client"
	"github.com/ks888/noisegate/common/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: filepath.Base(os.Args[0]),
		Commands: []*cli.Command{
			{
				Name:      "test",
				Aliases:   []string{"t"},
				Usage:     "Run a test",
				ArgsUsage: "[changed_file_path:#offset1,#offset2,... (e.g. sum.go:#0,#1-2,#4)]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the file path is not specified")
					}

					log.EnableDebugLog(c.Bool("debug"))

					query := c.Args().First()
					options := client.TestOptions{ServerAddr: c.String("addr"), TestLogger: os.Stdout, BuildTags: c.String("tags"), Bypass: c.Bool("bypass")}
					return client.TestAction(c.Context, query, options)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "tags",
						Usage: "a comma-separated list of build tags.",
						Value: "",
					},
					&cli.BoolFlag{
						Name:  "bypass",
						Usage: "do not exclude any tests.",
					},
				},
			},
			{
				Name:      "hint",
				Usage:     "Hint the recent change of the specified file",
				ArgsUsage: "[changed_file_path:#offset1,#offset2,... (e.g. sum.go:#0,#1-2,#4)]",
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