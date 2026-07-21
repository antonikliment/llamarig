package public_http

import (
	"encoding/json"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"llamarig/core/control"
	"llamarig/core/rpc"
)

func writeRPCMappedResponse[T any](w http.ResponseWriter, response *T, err error, nilMessage string, mapResponse func(*T) any) {
	if err != nil {
		writeRPCError(w, err)
		return
	}
	if response == nil {
		writeCoreError(w, control.CoreError(control.ErrorRuntime, nilMessage, nil))
		return
	}
	writeJSON(w, http.StatusOK, mapResponse(response))
}

func writeRPCError(w http.ResponseWriter, err error) {
	kind := rpc.ErrorKindFromRPC(err)
	message := err.Error()
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		message = connectErr.Message()
	}
	writeJSON(w, httpStatusForKind(kind), map[string]any{"ok": false, "error": map[string]any{"kind": kind, "message": message}})
}

func writeCoreError(w http.ResponseWriter, err error) {
	status := httpStatusForKind(control.Kind(err))
	writeJSON(w, status, map[string]any{"ok": false, "error": map[string]any{"kind": control.Kind(err), "message": control.Message(err)}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	var (
		data []byte
		err  error
	)
	data, err = json.Marshal(value)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"kind":"runtime","message":"failed to marshal response"}}`))
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func httpStatusForKind(kind control.ErrorKind) int {
	switch kind {
	case control.ErrorInvalidInput:
		return http.StatusBadRequest
	case control.ErrorPermission:
		return http.StatusForbidden
	case control.ErrorNotFound:
		return http.StatusNotFound
	case control.ErrorConflict:
		return http.StatusConflict
	case control.ErrorTimeout:
		return http.StatusGatewayTimeout
	case control.ErrorRuntime:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
