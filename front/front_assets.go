package front

import (
	"embed"
	"io/fs"
)

//go:embed build/*
var embedAssets embed.FS

var FrontAssets, _ = fs.Sub(embedAssets, "build")
