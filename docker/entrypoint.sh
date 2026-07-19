#!/bin/sh
# Container entrypoint for LlamaRig.
#
# The setup wizard only runs in an interactive terminal, so in a container we
# seed a default config.yaml + models.ini on first boot when the LLAMARIG_HOME
# volume is empty. Mount an existing ~/.llamarig to reuse your own config.
set -e

: "${LLAMARIG_HOME:=/root/.llamarig}"
: "${LLAMA_SERVER_BIN:=/app/llama-server}"
: "${LLAMARIG_LISTEN_ADDR:=0.0.0.0:7000}"
export LLAMARIG_HOME

config="$LLAMARIG_HOME/config.yaml"
models_ini="$LLAMARIG_HOME/models.ini"
models_dir="$LLAMARIG_HOME/models"

mkdir -p "$models_dir"

if [ ! -f "$config" ]; then
	echo "llamarig: no config found, writing defaults to $config" >&2
	cat >"$config" <<EOF
listen_addr: "$LLAMARIG_LISTEN_ADDR"
model_storage_dir: "$models_dir"
startup_services:
  - control
  - web

security:
  auth_token_env: "LLAMARIG_CONTROL_TOKEN"
  disable_origin_check: false

router:
  executable: "$LLAMA_SERVER_BIN"
  port: 8080
  models_max: 1
  default_preset: "default"
  autostart_presets: []
  stop_timeout: 10s
  env: {}
  readiness_timeout: 60s
  readiness_interval: 500ms
EOF
fi

if [ ! -f "$models_ini" ]; then
	cat >"$models_ini" <<EOF
[default]
models-dir = $models_dir
EOF
fi

# Run the two services: control daemon detached, web gateway in the foreground
# as PID 1 so the container lifecycle follows the gateway.
llamarig serve --detach
exec llamarig gateway --foreground
