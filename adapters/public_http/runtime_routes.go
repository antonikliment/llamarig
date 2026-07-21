package public_http

import (
	"context"
	"net/http"

	"llamarig/core/rpc"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if s.internalControl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": rpc.ServiceName})
		return
	}
	health, err := s.internalControl.Health(r.Context(), &controlv1.HealthRequest{})
	writeRPCMappedResponse(w, health, err, "internal health rpc returned no response", identity)
}

// rpcGet adapts a request-less control RPC into a GET handler.
func rpcGet[Req, Resp any](s *Server, call func(controlv1connect.ControlServiceClient, context.Context, *Req) (*Resp, error), nilMessage string, mapResponse func(*Resp) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.internalControl == nil {
			http.Error(w, "internal control service not available", http.StatusServiceUnavailable)
			return
		}
		response, err := call(s.internalControl, r.Context(), new(Req))
		writeRPCMappedResponse(w, response, err, nilMessage, mapResponse)
	}
}

func identity[T any](response *T) any { return response }
