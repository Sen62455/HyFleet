package webui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func Handler() (http.Handler, error) {
	assets, err := assetFS()
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServerFS(assets)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		info, statErr := fs.Stat(assets, name)
		if statErr != nil || info.IsDir() {
			clone := request.Clone(request.Context())
			clone.URL.Path = "/"
			response.Header().Set("Cache-Control", "no-cache")
			fileServer.ServeHTTP(response, clone)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			response.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(response, request)
	}), nil
}
