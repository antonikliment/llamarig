package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llamarig/config"
	"llamarig/platform/audit"
)

// TestGatewayDaemonLogs drives GET /api/logs end-to-end: the gateway tails the
// daemon log file (zap JSON interleaved with raw llama-server output) and
// returns it verbatim for the web UI to classify client-side.
func TestGatewayDaemonLogs(t *testing.T) {
	gw := startGateway(t)

	logPath, err := audit.GetLogPath(config.ProjectName)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"level":"info","ts":1782571471.83,"caller":"cmd/serve.go:48","msg":"starting control rpc","socket":"/run/control.sock"}`,
		`{"level":"error","ts":1782571473,"msg":"stop llama runtime","error":"router stop timed out"}`,
		`[45877] 0.07.763 I srv  llama_server: model loaded`,
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	body := gatewayJSON(t, gw, http.MethodGet, "/api/logs", "")
	if body["ok"] != true {
		t.Fatalf("/api/logs ok=false: %#v", body)
	}
	text, _ := body["text"].(string)
	for _, want := range []string{"starting control rpc", `"level":"error"`, "llama_server: model loaded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("/api/logs text missing %q:\n%s", want, text)
		}
	}
}

func gatewayJSON(t *testing.T, handler http.Handler, method, target, _ string) map[string]any {
	t.Helper()
	request := httptest.NewRequest(method, target, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s %s -> %d: %s", method, target, response.Code, response.Body.String())
	}
	result := map[string]any{}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
