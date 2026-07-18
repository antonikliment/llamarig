package public_http

import (
	"io/fs"
	platformconfig "llamarig/config"
	"net/http"
	"time"
)

func PublicHttpServer(cfg platformconfig.Config, appFS fs.FS, authToken string, socketPath string) *http.Server {
	httpDeps := Dependencies{AuthToken: authToken, AppFS: appFS, DisableOriginCheck: cfg.Security.DisableOriginCheck, InternalSocketPath: socketPath}
	return &http.Server{Addr: cfg.ListenAddr, Handler: NewServer(httpDeps).Handler(), ReadHeaderTimeout: 5 * time.Second}
}
