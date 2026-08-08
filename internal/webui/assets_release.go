//go:build webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var releaseFiles embed.FS

func assetFS() (fs.FS, error) {
	return fs.Sub(releaseFiles, "dist")
}
