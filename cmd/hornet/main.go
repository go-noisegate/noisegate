package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ks888/hornet/client"
	"github.com/ks888/hornet/common/log"
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
				ArgsUsage: "[changed_file_path:#offset (e.g. sum.go:#1)]",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return errors.New("the file path is not specified")
					}

					log.EnableDebugLog(c.Bool("debug"))

					query := c.Args().First()
					options := client.TestOptions{ServerAddr: c.String("addr"), TestLogger: os.Stdout, Parallel: c.String("parallel"), BuildTags: c.String("tags")}
					return client.TestAction(c.Context, query, options)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "parallel",
						Aliases: []string{"p"},
						Usage:   "enable the parallel testing [on, off or auto]. When `auto`, the tool automatically chooses the faster option.",
						Value:   "auto",
					},
					&cli.StringFlag{
						Name:  "tags",
						Usage: "a comma-separated list of build tags.",
						Value: "",
					},
				},
			},
			{
				Name:      "hint",
				Usage:     "Hint the recent change of the specified file",
				ArgsUsage: "[changed_file_path:#offset (e.g. sum.go:#1)]",
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
				Usage: "hornetd server's `address`",
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
