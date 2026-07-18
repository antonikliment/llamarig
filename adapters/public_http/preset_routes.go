package public_http

import (
	"net/http"

	controlv1 "llamarig/core/rpc/gen/v1"
)

type presetEntryBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type presetBody struct {
	Name    string            `json:"name"`
	Entries []presetEntryBody `json:"entries"`
}

func (s *Server) presetGet(w http.ResponseWriter, r *http.Request) {
	resp, err := s.internalControl.GetPreset(r.Context(), &controlv1.GetPresetRequest{Name: r.PathValue("name")})
	writeRPCResponse(w, resp, err, "internal preset rpc returned no response")
}

func (s *Server) presetCreate(w http.ResponseWriter, r *http.Request) {
	var body presetBody
	if !readJSONBody(w, r, &body) {
		return
	}
	resp, err := s.internalControl.PutPreset(r.Context(), &controlv1.PutPresetRequest{Preset: presetProtoFromBody(body.Name, body), CreateOnly: true})
	writeRPCResponse(w, resp, err, "internal preset rpc returned no response")
}

func (s *Server) presetReplace(w http.ResponseWriter, r *http.Request) {
	var body presetBody
	if !readJSONBody(w, r, &body) {
		return
	}
	resp, err := s.internalControl.PutPreset(r.Context(), &controlv1.PutPresetRequest{Preset: presetProtoFromBody(r.PathValue("name"), body), CreateOnly: false})
	writeRPCResponse(w, resp, err, "internal preset rpc returned no response")
}

func (s *Server) presetDelete(w http.ResponseWriter, r *http.Request) {
	_, err := s.internalControl.DeletePreset(r.Context(), &controlv1.DeletePresetRequest{Name: r.PathValue("name")})
	writeMutationResponse(w, err)
}

func (s *Server) presetCleanup(w http.ResponseWriter, r *http.Request) {
	resp, err := s.internalControl.CleanupPreset(r.Context(), &controlv1.CleanupPresetRequest{Name: r.PathValue("name")})
	writeRPCResponse(w, resp, err, "internal cleanup preset rpc returned no response")
}

type presetAutostartBody struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) presetSetAutostart(w http.ResponseWriter, r *http.Request) {
	var body presetAutostartBody
	if !readJSONBody(w, r, &body) {
		return
	}
	_, err := s.internalControl.SetPresetAutostart(r.Context(), &controlv1.PresetAutostartRequest{Name: r.PathValue("name"), Enabled: body.Enabled})
	writeMutationResponse(w, err)
}

func presetProtoFromBody(name string, body presetBody) *controlv1.ModelPreset {
	entries := make([]*controlv1.PresetEntry, 0, len(body.Entries))
	for _, entry := range body.Entries {
		entries = append(entries, &controlv1.PresetEntry{Key: entry.Key, Value: entry.Value})
	}
	return &controlv1.ModelPreset{Name: name, Entries: entries}
}
