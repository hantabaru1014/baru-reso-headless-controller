package front

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var embedAssets embed.FS

var FrontAssets, _ = fs.Sub(embedAssets, "dist")
