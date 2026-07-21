package public_http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"llamarig/config"
	"llamarig/platform/audit"
)

func TestHTTPLogTailAndArchiveLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, home)
	writeHTTPLog(t, config.ProjectName, "one\ntwo\nthree\n")
	server := NewServer(Dependencies{AuthToken: "secret", InternalSocketPath: filepath.Join(t.TempDir(), "missing.sock")})

	req := httptest.NewRequest(http.MethodGet, "/api/logs?source=control&lines=2", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() == "" {
		t.Fatalf("tail status=%d body=%s", rec.Code, rec.Body.String())
	}

	id, err := audit.ArchiveLog(config.ProjectName, time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/logs/archives", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	var listed struct {
		Archives []struct {
			ID string `json:"id"`
		} `json:"archives"`
	}
	server.Handler().ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed.Archives) != 1 || listed.Archives[0].ID != id {
		t.Fatalf("archives=%#v err=%v body=%s", listed.Archives, err, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/logs/archives/"+id, nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPLogRoutesRequireAuthAndValidateInput(t *testing.T) {
	t.Setenv(config.ProjectHomeEnv, t.TempDir())
	server := NewServer(Dependencies{AuthToken: "secret", InternalSocketPath: filepath.Join(t.TempDir(), "missing.sock")})
	unauthorized := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	if unauthorized.Code != http.StatusForbidden {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/logs?source=unknown&lines=0", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func writeHTTPLog(t *testing.T, name, text string) {
	t.Helper()
	path, err := audit.GetLogPath(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
