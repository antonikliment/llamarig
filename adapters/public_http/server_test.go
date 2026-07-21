package public_http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"

	"llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

type gatewayControl struct {
	controlv1connect.UnimplementedControlServiceHandler
}

func (gatewayControl) Health(context.Context, *controlv1.HealthRequest) (*controlv1.HealthResponse, error) {
	return &controlv1.HealthResponse{Ok: true, Service: "socket-rpc"}, nil
}

func (gatewayControl) SetStartupServices(_ context.Context, request *controlv1.SetStartupServicesRequest) (*controlv1.MutationResponse, error) {
	return &controlv1.MutationResponse{Ok: len(request.GetServices()) > 0}, nil
}

func (gatewayControl) WatchModelCatalog(_ context.Context, _ *controlv1.WatchModelCatalogRequest, stream *connect.ServerStream[controlv1.ModelCatalogEvent]) error {
	return stream.Send(&controlv1.ModelCatalogEvent{Type: "catalog_refresh", Ok: true})
}

func TestControlRPCProxy(t *testing.T) {
	server := newGatewayTestServer(t, gatewayControl{}, Dependencies{})
	public := httptest.NewServer(server.Handler())
	t.Cleanup(public.Close)
	client := controlv1connect.NewControlServiceClient(public.Client(), public.URL, connect.WithProtoJSON())

	response, err := client.Health(context.Background(), &controlv1.HealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.GetOk() || response.GetService() != "socket-rpc" {
		t.Fatalf("response = %#v", response)
	}
	stream, err := client.WatchModelCatalog(context.Background(), &controlv1.WatchModelCatalogRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !stream.Receive() || stream.Msg().GetType() != "catalog_refresh" {
		t.Fatalf("stream error = %v message = %#v", stream.Err(), stream.Msg())
	}
}

func TestControlRPCAuth(t *testing.T) {
	server := newGatewayTestServer(t, gatewayControl{}, Dependencies{AuthToken: "secret"})
	public := httptest.NewServer(server.Handler())
	t.Cleanup(public.Close)
	client := controlv1connect.NewControlServiceClient(public.Client(), public.URL)

	_, err := client.SetStartupServices(context.Background(), &controlv1.SetStartupServicesRequest{Services: []string{"gateway"}})
	if err == nil {
		t.Fatal("unauthorized mutation succeeded")
	}

	authorized := controlv1connect.NewControlServiceClient(public.Client(), public.URL, connect.WithInterceptors(bearerInterceptor("secret")))
	response, err := authorized.SetStartupServices(context.Background(), &controlv1.SetStartupServicesRequest{Services: []string{"gateway"}})
	if err != nil {
		t.Fatal(err)
	}
	if !response.GetOk() {
		t.Fatalf("response = %#v", response)
	}
}

func TestLegacyHealthAndInfoRoutes(t *testing.T) {
	server := newGatewayTestServer(t, gatewayControl{}, Dependencies{})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"service":"socket-rpc"`) {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("removed REST status = %d", response.Code)
	}
}

func TestServerServesAppWithoutShadowingAPIs(t *testing.T) {
	app := fstest.MapFS{"index.html": {Data: []byte("app")}, "assets/app.js": {Data: []byte("asset")}}
	server := NewServer(Dependencies{AppFS: app, InternalSocketPath: filepath.Join(t.TempDir(), "missing.sock")})

	for path, want := range map[string]int{"/": http.StatusOK, "/settings": http.StatusNotFound, "/assets/app.js": http.StatusOK, "/api/removed": http.StatusNotFound} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want {
			t.Errorf("%s status = %d, want %d", path, response.Code, want)
		}
	}
}

func TestServerRejectsUntrustedOrigin(t *testing.T) {
	server := NewServer(Dependencies{InternalSocketPath: filepath.Join(t.TempDir(), "missing.sock")})
	request := httptest.NewRequest(http.MethodPost, controlv1connect.ControlServiceHealthProcedure, nil)
	request.Host = "127.0.0.1:7105"
	request.Header.Set("Origin", "http://evil.example")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestMCPAuthProtectsEndpoint(t *testing.T) {
	server := NewServer(Dependencies{AuthToken: "secret", InternalSocketPath: filepath.Join(t.TempDir(), "missing.sock")})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func newGatewayTestServer(t *testing.T, service controlv1connect.ControlServiceHandler, dependencies Dependencies) *Server {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	path, handler := controlv1connect.NewControlServiceHandler(service)
	internal := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, path) {
			t.Errorf("unexpected RPC path %q", r.URL.Path)
		}
		handler.ServeHTTP(w, r)
	})}
	done := make(chan error, 1)
	go func() { done <- internal.Serve(listener) }()
	t.Cleanup(func() {
		_ = internal.Close()
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	})
	dependencies.InternalSocketPath = socketPath
	return NewServer(dependencies)
}

func bearerInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			request.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, request)
		}
	}
}
