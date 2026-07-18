package runtime

import (
	"strconv"

	platformconfig "llamarig/config"
)

// BuildRouter configures one supervised llama-server router process.
func BuildRouter(global platformconfig.RouterConfig, modelsDir, modelsPreset string) *LlamaServer {
	return NewLlamaServer(LlamaServerConfig{Name: "router", Executable: global.Executable, Argv: []string{"--models-dir", modelsDir, "--models-preset", modelsPreset, "--models-max", strconv.Itoa(global.ModelsMax), "--host", global.Host, "--port", strconv.Itoa(global.Port)}, Host: global.Host, Port: global.Port, Env: global.Env, Timeout: global.StopTimeout, ReadinessPath: "/health", ReadinessTimeout: global.ReadinessTimeout, ReadinessInterval: global.ReadinessInterval})
}
