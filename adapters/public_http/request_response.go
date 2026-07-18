package public_http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"connectrpc.com/connect"

	"llamarig/core/control"
	"llamarig/core/rpc"
	controlv1 "llamarig/core/rpc/gen/v1"
)

const requestBodyLimitBytes int64 = 2 << 20

func readJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	data, ok := readLimitedBody(w, r)
	if !ok {
		return false
	}
	if err := json.Unmarshal(data, dst); err != nil {
		writeCoreError(w, control.CoreError(control.ErrorInvalidInput, "decode request body", err))
		return false
	}
	return true
}

func readLimitedBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.Body == nil {
		writeCoreError(w, control.Errorf(control.ErrorInvalidInput, "request body is missing"))
		return nil, false
	}
	defer func() { _ = r.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(r.Body, requestBodyLimitBytes+1))
	if err != nil {
		writeCoreError(w, control.CoreError(control.ErrorInvalidInput, "read request body", err))
		return nil, false
	}
	if int64(len(data)) > requestBodyLimitBytes {
		writeCoreError(w, control.Errorf(control.ErrorInvalidInput, "request body exceeds %d byte limit", requestBodyLimitBytes))
		return nil, false
	}
	return data, true
}

func writeCommandRPCResponse(w http.ResponseWriter, response *controlv1.CommandResponse, err error) {
	writeRPCResponse(w, response, err, "internal command rpc returned no response")
}

func writeModelDownloadResponse(w http.ResponseWriter, response *controlv1.ModelDownloadResponse, err error) {
	writeRPCResponse(w, response, err, "internal model download rpc returned no response")
}

func writeRPCSuccess(w http.ResponseWriter, err error, success any) {
	if err != nil {
		writeRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, success)
}

func writeMutationResponse(w http.ResponseWriter, err error) {
	writeRPCSuccess(w, err, map[string]any{"ok": true, "result": map[string]any{}})
}

func writeRPCResponse[T any](w http.ResponseWriter, response *T, err error, nilMessage string) {
	writeRPCMappedResponse(w, response, err, nilMessage, func(response *T) any { return response })
}

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

func writeCoreError(w http.ResponseWriter, err error) {
	status := httpStatusForKind(control.Kind(err))
	writeJSON(w, status, map[string]any{"ok": false, "error": map[string]any{"kind": control.Kind(err), "message": control.Message(err)}})
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
