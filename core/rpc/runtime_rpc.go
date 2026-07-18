package rpc

import (
	"context"
	"llamarig/core/control"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/signals"
)

func (s *ControlService) GetRuntimeStatus(ctx context.Context, _ *controlv1.GetRuntimeStatusRequest) (*controlv1.GetRuntimeStatusResponse, error) {
	status, err := s.manager.Status(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.GetRuntimeStatusResponse{Ok: true, Status: runtimeStatusProto(status)}, nil
}

func (s *ControlService) GetRuntimeResources(ctx context.Context, _ *controlv1.GetRuntimeResourcesRequest) (*controlv1.GetRuntimeResourcesResponse, error) {
	snapshot, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &controlv1.GetRuntimeResourcesResponse{Ok: true, Resources: runtimeResourcesProto(snapshot)}, nil
}

// snapshot captures a signals snapshot, returning a mapped RPC error when the
// collector is unconfigured or the capture fails.
func (s *ControlService) snapshot(ctx context.Context) (signals.Snapshot, error) {
	if s.signals == nil {
		return signals.Snapshot{}, rpcError(control.Errorf(control.ErrorInvalidInput, "signals collector is not configured"))
	}
	snapshot, err := s.signals.Snapshot(ctx)
	if err != nil {
		return signals.Snapshot{}, rpcError(err)
	}
	return snapshot, nil
}

func (s *ControlService) GetLlamaServerParams(ctx context.Context, _ *controlv1.GetLlamaServerParamsRequest) (*controlv1.GetLlamaServerParamsResponse, error) {
	params, err := s.manager.GetLlamaServerParams(ctx)
	if err != nil {
		return &controlv1.GetLlamaServerParamsResponse{Ok: false, Warning: err.Error()}, nil
	}
	return &controlv1.GetLlamaServerParamsResponse{Ok: true, Params: mapProto(params, func(p control.LlamaServerParam) *controlv1.LlamaServerParam {
		return &controlv1.LlamaServerParam{Key: p.Key, Aliases: p.Aliases, ValueHint: p.ValueHint, DefaultValue: p.Default, Description: p.Description}
	}), Source: "llama-server --help"}, nil
}

func (s *ControlService) StartRuntime(ctx context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return runtimeOp(ctx, req.GetTarget(), s.manager.StartOperation)
}

func (s *ControlService) StopRuntime(ctx context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return runtimeOp(ctx, req.GetTarget(), s.manager.StopOperation)
}

func (s *ControlService) RestartRuntime(ctx context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return runtimeOp(ctx, req.GetTarget(), s.manager.RestartOperation)
}

func runtimeOp(ctx context.Context, target string, op func(context.Context, string) (control.OperationResult, error)) (*controlv1.CommandResponse, error) {
	result, err := op(ctx, target)
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.CommandResponse{Ok: true, Result: operationResultProto(result)}, nil
}
