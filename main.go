package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "tgo",
		Usage: "A simple task manager",
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
}
