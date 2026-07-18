package cli

import (
	"time"

	"llamarig/core/rpc"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

const clientTimeout = 30 * time.Second

func (c command) controlClient() (controlv1connect.ControlServiceClient, error) {
	return rpc.DialControl(c.socket, clientTimeout)
}
