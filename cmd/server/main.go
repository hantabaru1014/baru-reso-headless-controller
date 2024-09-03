package main

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/app"
)

func main() {
	s := app.InitializeServer()
	s.ListenAndServe(":8014")
}
