package mcp

import (
	"net/http"

	"llamarig/core/rpc/gen/v1/controlv1connect"
	"llamarig/internal/buildinfo"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Dependencies struct {
	ControlClient controlv1connect.ControlServiceClient
	ServiceName   string
}

func NewServer(deps Dependencies) *mcp.Server {
	if deps.ControlClient == nil {
		panic("mcp: ControlClient dependency is required")
	}
	server := mcp.NewServer(&mcp.Implementation{Name: deps.ServiceName, Version: buildinfo.Version}, nil)
	addTools(server, deps.ControlClient)
	addResources(server, deps.ControlClient)
	return server
}

func NewHandler(deps Dependencies) http.Handler {
	server := NewServer(deps)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)
}
