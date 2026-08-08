//go:build !webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed fallback/*
var fallbackFiles embed.FS

func assetFS() (fs.FS, error) {
	return fs.Sub(fallbackFiles, "fallback")
}
