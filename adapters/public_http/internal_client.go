package public_http

import (
	"time"

	"llamarig/core/rpc"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

func newInternalControlClient(socketPath string) controlv1connect.ControlServiceClient {
	client, _ := rpc.DialControl(socketPath, 30*time.Second)
	return client
}

func newInternalStreamingControlClient(socketPath string) controlv1connect.ControlServiceClient {
	client, _ := rpc.DialControl(socketPath, 0)
	return client
}
