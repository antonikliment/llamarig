package public_http

import (
	"llamarig/adapters/mcp"
	"llamarig/core/rpc"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
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
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/info", info)
	s.mux.HandleFunc("GET /api/runtime/status", rpcGet(s, controlv1connect.ControlServiceClient.GetRuntimeStatus, "internal runtime status rpc returned no response", func(status *controlv1.GetRuntimeStatusResponse) any { return status.GetStatus() }))
	s.mux.HandleFunc("GET /api/runtime/llama-params", s.requireAuth(rpcGet(s, controlv1connect.ControlServiceClient.GetLlamaServerParams, "internal llama server params rpc returned no response", llamaParamsPayload)))
	s.mux.HandleFunc("GET /api/signals", rpcGet(s, controlv1connect.ControlServiceClient.GetSignals, "internal signals rpc returned no response", identity))
	s.mux.HandleFunc("GET /api/events", rpcGet(s, controlv1connect.ControlServiceClient.ListEvents, "internal events rpc returned no response", identity))
	s.mux.HandleFunc("GET /api/logs", s.requireAuth(s.daemonLogs))
	s.mux.HandleFunc("GET /api/logs/archives", s.requireAuth(s.logArchives))
	s.mux.HandleFunc("DELETE /api/logs/archives", s.requireAuth(s.logArchivesClear))
	s.mux.HandleFunc("GET /api/logs/archives/{id}", s.requireAuth(s.logArchive))
	s.mux.HandleFunc("DELETE /api/logs/archives/{id}", s.requireAuth(s.logArchiveDelete))
	s.mux.HandleFunc("POST /api/runtime/start", s.requireAuth(runtimeAction(s, controlv1connect.ControlServiceClient.StartRuntime)))
	s.mux.HandleFunc("POST /api/runtime/stop", s.requireAuth(runtimeAction(s, controlv1connect.ControlServiceClient.StopRuntime)))
	s.mux.HandleFunc("POST /api/runtime/restart", s.requireAuth(runtimeAction(s, controlv1connect.ControlServiceClient.RestartRuntime)))
	s.mux.HandleFunc("GET /api/presets", rpcGet(s, controlv1connect.ControlServiceClient.ListPresets, "internal presets rpc returned no response", identity))
	s.mux.HandleFunc("POST /api/presets", s.requireAuth(s.presetCreate))
	s.mux.HandleFunc("GET /api/presets/{name}", s.presetGet)
	s.mux.HandleFunc("PUT /api/presets/{name}", s.requireAuth(s.presetReplace))
	s.mux.HandleFunc("DELETE /api/presets/{name}", s.requireAuth(s.presetDelete))
	s.mux.HandleFunc("POST /api/presets/{name}/cleanup", s.requireAuth(s.presetCleanup))
	s.mux.HandleFunc("POST /api/presets/{name}/autostart", s.requireAuth(s.presetSetAutostart))
	s.mux.HandleFunc("POST /api/models/resolve", s.requireAuth(s.modelResolve))
	s.mux.HandleFunc("GET /api/models/catalog", s.modelCatalogList)
	s.mux.HandleFunc("GET /api/models/local", rpcGet(s, controlv1connect.ControlServiceClient.ListLocalModels, "internal list local models rpc returned no response", identity))
	s.mux.HandleFunc("DELETE /api/models/local", s.requireAuth(s.localModelDelete))
	s.mux.HandleFunc("GET /api/models/catalog/events", s.modelCatalogEvents)
	s.mux.HandleFunc("POST /api/models/downloads", s.requireAuth(s.modelDownloadStart))
	s.mux.HandleFunc("GET /api/models/downloads/{id}", s.modelDownloadGet)
	s.mux.HandleFunc("DELETE /api/models/downloads/{id}", s.requireAuth(s.modelDownloadCancel))
	s.mux.HandleFunc("POST /api/models/downloads/{id}/apply-to-preset", s.requireAuth(s.modelApplyToPreset))

	mcpHandler := mcp.NewHandler(mcp.Dependencies{ControlClient: s.internalControl, ServiceName: rpc.ServiceName})
	if s.authToken != "" {
		mcpHandler = s.requireAuthHandler(mcpHandler)
	}
	s.mux.Handle("/mcp", mcpHandler)
}
