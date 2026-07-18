package rpc

import (
	"context"
	"errors"
	"fmt"
	platformconfig "llamarig/config"
	"llamarig/core/rpc/gen/v1/controlv1connect"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"connectrpc.com/connect"
)

type ControlRPCServer struct {
	Server     *http.Server
	Listener   net.Listener
	SocketPath string
}

func NewControlRPCServer(deps RPCDependencies) (ControlRPCServer, error) {
	listener, socketPath, err := newControlRPCListener()
	if err != nil {
		return ControlRPCServer{}, err
	}

	path, handler := controlv1connect.NewControlServiceHandler(
		NewControlService(deps),
		connect.WithInterceptors(validateRequestInterceptor()),
	)
	return ControlRPCServer{
		Server: &http.Server{
			Addr:              "",
			Handler:           controlRPCHandler(path, handler),
			ReadHeaderTimeout: 5 * time.Second,
		},
		Listener:   listener,
		SocketPath: socketPath,
	}, nil
}

func controlRPCHandler(path string, handler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	return mux
}

func newControlRPCListener() (net.Listener, string, error) {
	path, err := platformconfig.ControlSocketPath()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create control rpc socket dir: %w", err)
	}
	// MkdirAll does not tighten permissions on an existing directory.
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, "", fmt.Errorf("chmod control rpc socket dir: %w", err)
	}
	if err := removeStaleSocket(path); err != nil {
		return nil, "", err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", fmt.Errorf("listen control rpc socket %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, "", fmt.Errorf("chmod control rpc socket %q: %w", path, err)
	}
	return listener, path, nil
}

// DialControl builds a ControlService client over the local control Unix
// socket. Callers (CLI, TUI) only differ in the request timeout.
func DialControl(socketPath string, timeout time.Duration) (controlv1connect.ControlServiceClient, error) {
	if socketPath == "" {
		path, err := platformconfig.ControlSocketPath()
		if err != nil {
			return nil, err
		}
		socketPath = path
	}
	client := &http.Client{Timeout: timeout}
	client.Transport = &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socketPath)
		},
		DisableKeepAlives: true,
	}
	return controlv1connect.NewControlServiceClient(client, "http://unix"), nil
}

func removeStaleSocket(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat control rpc socket %q: %w", path, err)
	}
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("control rpc socket %q is already in use", path)
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return fmt.Errorf("control rpc socket %q dial timed out (likely in use): %w", path, err)
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("control rpc socket %q is already in use (permission denied)", path)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale control rpc socket %q: %w", path, err)
	}
	return nil
}
