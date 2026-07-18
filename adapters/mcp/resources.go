package mcp

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addResources(server *mcp.Server, client controlv1connect.ControlServiceClient) {
	read := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return readResource(ctx, client, req.Params.URI)
	}
	server.AddResource(&mcp.Resource{URI: "presets://", Name: "presets", MIMEType: "application/json"}, read)
	server.AddResourceTemplate(&mcp.ResourceTemplate{URITemplate: "presets://{name}", Name: "model preset", MIMEType: "application/json"}, read)
}

func readResource(ctx context.Context, client controlv1connect.ControlServiceClient, uri string) (*mcp.ReadResourceResult, error) {
	if uri == "presets://" {
		return readPresetListResource(ctx, client, uri)
	}
	name, ok := parsePresetResourceURI(uri)
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	out, err := client.GetPreset(ctx, &controlv1.GetPresetRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return jsonResource(uri, presetMap(out.GetPreset()))
}

func readPresetListResource(ctx context.Context, client controlv1connect.ControlServiceClient, uri string) (*mcp.ReadResourceResult, error) {
	out, err := client.ListPresets(ctx, &controlv1.ListPresetsRequest{})
	if err != nil {
		return nil, err
	}
	return jsonResource(uri, presetMaps(out.GetPresets()))
}

func parsePresetResourceURI(raw string) (string, bool) {
	const prefix = "presets://"
	encoded, ok := strings.CutPrefix(raw, prefix)
	if !ok || encoded == "" || strings.ContainsAny(encoded, "/?#") {
		return "", false
	}
	name, err := url.PathUnescape(encoded)
	if err != nil {
		return "", false
	}
	return name, true
}

func jsonResource(uri string, payload any) (*mcp.ReadResourceResult, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return textResource(uri, "application/json", string(data)), nil
}

func textResource(uri string, mime string, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mime, Text: text}}}
}
