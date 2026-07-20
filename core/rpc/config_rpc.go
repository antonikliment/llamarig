package rpc

import (
	"context"
	controlv1 "llamarig/core/rpc/gen/v1"
)

func (s *ControlService) SetStartupServices(ctx context.Context, req *controlv1.SetStartupServicesRequest) (*controlv1.MutationResponse, error) {
	_, err := s.manager.SetStartupServices(ctx, req.GetServices())
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.MutationResponse{Ok: true}, nil
}
