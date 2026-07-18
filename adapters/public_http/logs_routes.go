package public_http

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	platformconfig "llamarig/config"
	"llamarig/core/control"
	"llamarig/platform/audit"
)

// daemonLogs returns the tail of the control daemon's log file as raw text.
// The file interleaves zap JSON lines with raw llama-server output; the web UI
// classifies and colorizes them client-side.
func (s *Server) daemonLogs(w http.ResponseWriter, r *http.Request) {
	name, source, ok := logSource(r.URL.Query().Get("source"))
	if !ok {
		writeCoreError(w, control.Errorf(control.ErrorInvalidInput, "log source must be control or gateway"))
		return
	}
	lines, ok := logLines(r.URL.Query().Get("lines"))
	if !ok {
		writeCoreError(w, control.Errorf(control.ErrorInvalidInput, "log lines must be between 1 and %d", audit.MaxTailLines))
		return
	}
	text, err := audit.TailLogLines(name, lines)
	if err != nil {
		writeCoreError(w, control.CoreError(control.ErrorRuntime, "read daemon log", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source, "text": text})
}

func (s *Server) logArchives(w http.ResponseWriter, _ *http.Request) {
	archives, err := audit.ListArchives()
	if err != nil {
		writeCoreError(w, control.CoreError(control.ErrorRuntime, "list log archives", err))
		return
	}
	items := make([]map[string]any, 0, len(archives))
	for _, archive := range archives {
		_, source, ok := logSource(archive.Service)
		if !ok {
			continue
		}
		items = append(items, map[string]any{"id": archive.ID, "source": source, "size_bytes": archive.SizeBytes, "archived_at": archive.ArchivedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "archives": items})
}

func (s *Server) logArchive(w http.ResponseWriter, r *http.Request) {
	lines, ok := logLines(r.URL.Query().Get("lines"))
	if !ok {
		writeCoreError(w, control.Errorf(control.ErrorInvalidInput, "log lines must be between 1 and %d", audit.MaxTailLines))
		return
	}
	text, err := audit.TailArchive(r.PathValue("id"), lines)
	if err != nil {
		writeLogArchiveError(w, "read log archive", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": text})
}

func (s *Server) logArchiveDelete(w http.ResponseWriter, r *http.Request) {
	if err := audit.DeleteArchive(r.PathValue("id")); err != nil {
		writeLogArchiveError(w, "delete log archive", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": 1})
}

func (s *Server) logArchivesClear(w http.ResponseWriter, _ *http.Request) {
	deleted, err := audit.ClearArchives()
	if err != nil {
		writeCoreError(w, control.CoreError(control.ErrorRuntime, "clear log archives", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": deleted})
}

func logSource(source string) (string, string, bool) {
	switch source {
	case "", "control", platformconfig.ProjectName:
		return platformconfig.ProjectName, "control", true
	case "gateway":
		return "gateway", "gateway", true
	default:
		return "", "", false
	}
}

func logLines(raw string) (int, bool) {
	if raw == "" {
		return audit.DefaultTailLines, true
	}
	lines, err := strconv.Atoi(raw)
	return lines, err == nil && lines >= 1 && lines <= audit.MaxTailLines
}

func writeLogArchiveError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeCoreError(w, control.Errorf(control.ErrorNotFound, "%s: %v", action, err))
		return
	}
	writeCoreError(w, control.CoreError(control.ErrorRuntime, action, err))
}
