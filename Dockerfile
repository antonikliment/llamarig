# syntax=docker/dockerfile:1

# LlamaRig container image.
#
# The runtime layer is built on top of the official llama.cpp server image so
# that `llama-server` (and its shared libraries) are present without compiling
# llama.cpp ourselves. Swap the base with --build-arg LLAMA_IMAGE=... to target
# a GPU build, e.g. ghcr.io/ggml-org/llama.cpp:server-cuda (see README).
ARG LLAMA_IMAGE=ghcr.io/ggml-org/llama.cpp:server

# --- Stage 1: build the web UI (embedded into the Go binary) ---
FROM node:24-bookworm-slim AS webui
WORKDIR /src/webui
RUN corepack enable && corepack prepare pnpm@10.22.0 --activate
COPY webui/package.json webui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY webui/ ./
RUN pnpm run build

# --- Stage 2: build the llamarig binary (embeds webui/dist) ---
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webui /src/webui/dist ./webui/dist
# core/rpc/gen/v1 is generated (gitignored), so regenerate before building.
RUN go generate ./core/rpc/gen \
    && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/llamarig .

# --- Stage 3: runtime on the official llama.cpp server image ---
FROM ${LLAMA_IMAGE} AS runtime
COPY --from=build /out/llamarig /usr/local/bin/llamarig
COPY docker/entrypoint.sh /usr/local/bin/llamarig-entrypoint
RUN chmod +x /usr/local/bin/llamarig-entrypoint
ENV LLAMARIG_HOME=/root/.llamarig \
    LLAMA_SERVER_BIN=/app/llama-server
EXPOSE 7000
ENTRYPOINT ["/usr/local/bin/llamarig-entrypoint"]
