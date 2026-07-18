package public_http

import (
	"io/fs"
	"net/http"
)

func appHandler(appFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(appFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, appFS, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
