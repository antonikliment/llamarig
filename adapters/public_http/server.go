package public_http

import (
	"io/fs"
	"net/http"
	"strings"

	"llamarig/core/rpc/gen/v1/controlv1connect"
)

type Server struct {
	mux                *http.ServeMux
	authToken          string
	app                http.Handler
	disableOriginCheck bool
	internalControl    controlv1connect.ControlServiceClient
	internalStream     controlv1connect.ControlServiceClient
}

type Dependencies struct {
	AuthToken          string
	AppFS              fs.FS
	DisableOriginCheck bool
	InternalSocketPath string
}

func NewServer(deps Dependencies) *Server {
	s := &Server{mux: http.NewServeMux(), authToken: deps.AuthToken, disableOriginCheck: deps.DisableOriginCheck}
	if deps.AppFS != nil {
		s.app = appHandler(deps.AppFS)
	}
	s.internalControl = newInternalControlClient(deps.InternalSocketPath)
	s.internalStream = newInternalStreamingControlClient(deps.InternalSocketPath)
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	handler := http.HandlerFunc(s.serveHTTP)
	if s.disableOriginCheck {
		return handler
	}
	return originGuard(handler)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if s.app != nil && r.Method == http.MethodGet && r.URL.Path != "/health" && r.URL.Path != "/info" && r.URL.Path != "/mcp" && r.URL.Path != "/api" && !strings.HasPrefix(r.URL.Path, "/api/") {
		s.app.ServeHTTP(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}
