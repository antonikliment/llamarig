package public_http

import (
	"llamarig/adapters/mcp"
	"llamarig/core/control"
	"llamarig/core/rpc"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
	"net/http"
)

func (s *Server) routes() {
	info := rpcGet(s, controlv1connect.ControlServiceClient.GetInfo, "internal info rpc returned no response", func(response *controlv1.GetInfoResponse) any {
		return struct {
			*controlv1.RuntimeInfo
			Build *controlv1.BuildInfo `json:"build"`
		}{RuntimeInfo: response.GetInfo(), Build: response.GetBuild()}
	})
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("GET /info", info)
	s.mux.HandleFunc("GET /api/logs", s.requireAuth(s.daemonLogs))
	s.mux.HandleFunc("GET /api/logs/archives", s.requireAuth(s.logArchives))
	s.mux.HandleFunc("DELETE /api/logs/archives", s.requireAuth(s.logArchivesClear))
	s.mux.HandleFunc("GET /api/logs/archives/{id}", s.requireAuth(s.logArchive))
	s.mux.HandleFunc("DELETE /api/logs/archives/{id}", s.requireAuth(s.logArchiveDelete))
	s.mux.Handle("/"+controlv1connect.ControlServiceName+"/", s.controlRPCAuth(s.internalRPC))

	mcpHandler := mcp.NewHandler(mcp.Dependencies{ControlClient: s.internalControl, ServiceName: rpc.ServiceName})
	if s.authToken != "" {
		mcpHandler = s.requireAuthHandler(mcpHandler)
	}
	s.mux.Handle("/mcp", mcpHandler)
}

func (s *Server) controlRPCAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicControlProcedure(r.URL.Path) || bearerTokenMatches(r.Header.Get("Authorization"), s.authToken) {
			next.ServeHTTP(w, r)
			return
		}
		writeCoreError(w, control.Errorf(control.ErrorPermission, "authorization required"))
	})
}

// publicControlProcedure reports whether a control procedure is readable
// without authentication. This allowlist is the single authorization source:
// any procedure not listed here requires a bearer token (see controlRPCAuth).
func publicControlProcedure(path string) bool {
	switch path {
	case controlv1connect.ControlServiceHealthProcedure,
		controlv1connect.ControlServiceGetInfoProcedure,
		controlv1connect.ControlServiceGetRuntimeStatusProcedure,
		controlv1connect.ControlServiceGetSignalsProcedure,
		controlv1connect.ControlServiceListEventsProcedure,
		controlv1connect.ControlServiceWatchEventsProcedure,
		controlv1connect.ControlServiceListPresetsProcedure,
		controlv1connect.ControlServiceGetPresetProcedure,
		controlv1connect.ControlServiceListModelCatalogProcedure,
		controlv1connect.ControlServiceListLocalModelsProcedure,
		controlv1connect.ControlServiceWatchModelCatalogProcedure,
		controlv1connect.ControlServiceGetModelDownloadProcedure:
		return true
	default:
		return false
	}
}
