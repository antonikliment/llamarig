package tabs

import (
	"time"

	"llamarig/core/rpc"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

const controlClientTimeout = 5 * time.Second

func newControlClient() (controlv1connect.ControlServiceClient, error) {
	return rpc.DialControl("", controlClientTimeout)
}
