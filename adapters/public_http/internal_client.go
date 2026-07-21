package public_http

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"llamarig/core/control"
	"llamarig/core/rpc"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

func newInternalControlClient(socketPath string) controlv1connect.ControlServiceClient {
	client, _ := rpc.DialControl(socketPath, 30*time.Second)
	return client
}

func newInternalControlProxy(socketPath string) http.Handler {
	transport, err := rpc.ControlTransport(socketPath)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeCoreError(w, control.CoreError(control.ErrorRuntime, "configure control rpc proxy", err))
		})
	}
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "unix"})
	proxy.Transport = transport
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeCoreError(w, control.CoreError(control.ErrorRuntime, "proxy control rpc", err))
	}
	return proxy
}
