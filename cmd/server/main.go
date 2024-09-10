package main

import (
	"flag"
	"log/slog"
	"os"
	"strings"

	"github.com/hantabaru1014/baru-reso-headless-controller/app"
)

var (
	hostAddress       = flag.String("host", ":8014", "The address to serve the server")
	isFrontDev        = flag.Bool("fdev", false, "Whether to use the front-end development server")
	frontDevServerUrl = flag.String("fdev-url", "http://localhost:5173", "The URL of the front-end development server")
)

func init() {
	// 環境変数からも読み込む
	flag.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv(strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")); s != "" {
			f.Value.Set(s)
		}
	})
	flag.Parse()
}

func main() {
	s := app.InitializeServer()

	frontURL := ""
	if isFrontDev != nil && *isFrontDev {
		frontURL = *frontDevServerUrl
		slog.Info("Using front-end development server", "url", frontURL)
	}

	s.ListenAndServe(*hostAddress, frontURL)
}
