package public_http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"llamarig/core/control"
	controlv1 "llamarig/core/rpc/gen/v1"
)

type modelResolveRequest struct {
	URL string `json:"url"`
}

type modelDownloadRequest struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Force    bool   `json:"force"`
}

type modelApplyRequest struct {
	Preset  string `json:"preset"`
	Preview bool   `json:"preview"`
}

func (s *Server) modelResolve(w http.ResponseWriter, r *http.Request) {
	var req modelResolveRequest
	if !readJSONBody(w, r, &req) {
		return
	}
	resolution, err := s.internalControl.ResolveModel(r.Context(), &controlv1.ResolveModelRequest{Url: req.URL})
	writeRPCMappedResponse(w, resolution, err, "internal resolve model rpc returned no response", func(resolution *controlv1.ResolveModelResponse) any {
		return resolution.GetResolution()
	})
}

func (s *Server) modelCatalogList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	catalog, err := s.internalControl.ListModelCatalog(r.Context(), &controlv1.ListModelCatalogRequest{Limit: int32(limit), Sort: query.Get("sort"), Search: query.Get("search"), MinFit: query.Get("min_fit"), Task: query.Get("task")})
	writeRPCResponse(w, catalog, err, "internal list model catalog rpc returned no response")
}

func (s *Server) localModelDelete(w http.ResponseWriter, r *http.Request) {
	cascade, _ := strconv.ParseBool(r.URL.Query().Get("cascade_presets"))
	result, err := s.internalControl.DeleteLocalModel(r.Context(), &controlv1.DeleteLocalModelRequest{Path: r.URL.Query().Get("path"), CascadePresets: cascade})
	writeRPCResponse(w, result, err, "internal delete local model rpc returned no response")
}

func (s *Server) modelCatalogEvents(w http.ResponseWriter, r *http.Request) {
	stream, err := s.internalStream.WatchModelCatalog(r.Context(), &controlv1.WatchModelCatalogRequest{})
	if err != nil {
		writeRPCError(w, err)
		return
	}
	if stream == nil {
		writeCoreError(w, control.Errorf(control.ErrorRuntime, "internal model catalog events rpc returned no stream"))
		return
	}
	defer func() { _ = stream.Close() }()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeCoreError(w, control.Errorf(control.ErrorInvalidInput, "streaming is not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for stream.Receive() {
		event := stream.Msg()
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.GetType(), data)
		flusher.Flush()
	}
}

func (s *Server) modelDownloadStart(w http.ResponseWriter, r *http.Request) {
	var req modelDownloadRequest
	if !readJSONBody(w, r, &req) {
		return
	}
	download, err := s.internalControl.StartModelDownload(r.Context(), &controlv1.StartModelDownloadRequest{Url: req.URL, Filename: req.Filename, Force: req.Force})
	writeModelDownloadResponse(w, download, err)
}

func (s *Server) modelDownloadGet(w http.ResponseWriter, r *http.Request) {
	download, err := s.internalControl.GetModelDownload(r.Context(), &controlv1.GetModelDownloadRequest{Id: r.PathValue("id")})
	writeModelDownloadResponse(w, download, err)
}

func (s *Server) modelDownloadCancel(w http.ResponseWriter, r *http.Request) {
	download, err := s.internalControl.CancelModelDownload(r.Context(), &controlv1.CancelModelDownloadRequest{Id: r.PathValue("id")})
	writeModelDownloadResponse(w, download, err)
}

func (s *Server) modelApplyToPreset(w http.ResponseWriter, r *http.Request) {
	var req modelApplyRequest
	if !readJSONBody(w, r, &req) {
		return
	}
	result, err := s.internalControl.ApplyModelDownloadToPreset(r.Context(), &controlv1.ApplyModelDownloadToPresetRequest{Id: r.PathValue("id"), Preset: req.Preset, Preview: req.Preview})
	if err != nil {
		writeRPCError(w, err)
		return
	}
	if result == nil {
		writeCoreError(w, control.Errorf(control.ErrorRuntime, "internal apply model rpc returned no response"))
		return
	}
	if req.Preview {
		preview := result.GetPreviewDiff()
		if preview == nil {
			writeCoreError(w, control.Errorf(control.ErrorRuntime, "preview diff is missing from response"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "preview": map[string]string{"original": preview.GetOriginal(), "updated": preview.GetUpdated()}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result.GetResult()})
}
