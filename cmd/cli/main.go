package main

import (
	"os"

	"github.com/hantabaru1014/baru-reso-headless-controller/app"
)

func main() {
	cli := app.InitializeCli()
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
