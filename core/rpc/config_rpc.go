package rpc

import (
	"context"
	controlv1 "llamarig/core/rpc/gen/v1"
	"time"
)

func (s *ControlService) GetConfig(ctx context.Context, _ *controlv1.GetConfigRequest) (*controlv1.TextDocumentResponse, error) {
	doc, err := s.manager.GetConfigYAML(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.TextDocumentResponse{Ok: true, Content: doc.Content, Path: doc.Path, SizeBytes: doc.SizeBytes, ModTime: doc.ModTime.Format(time.RFC3339), Sha256: doc.SHA256}, nil
}
func (s *ControlService) ValidateConfig(ctx context.Context, req *controlv1.ValidateTextDocumentRequest) (*controlv1.ValidationResponse, error) {
	err := s.manager.ValidateConfigYAML(ctx, req.GetContent())
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.ValidationResponse{Ok: true}, nil
}

func (s *ControlService) ReplaceConfig(ctx context.Context, req *controlv1.ReplaceTextDocumentRequest) (*controlv1.MutationResponse, error) {
	_, err := s.manager.ReplaceConfigYAML(ctx, req.GetContent())
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.MutationResponse{Ok: true}, nil
}
