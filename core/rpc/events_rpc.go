package rpc

import (
	"context"

	"connectrpc.com/connect"

	"llamarig/core/control"
	controlv1 "llamarig/core/rpc/gen/v1"
)

func (s *ControlService) ListEvents(_ context.Context, _ *controlv1.ListEventsRequest) (*controlv1.ListEventsResponse, error) {
	if s.events == nil {
		return &controlv1.ListEventsResponse{Ok: true}, nil
	}
	return &controlv1.ListEventsResponse{Ok: true, Events: mapProto(s.events.List(), func(event control.Event) *controlv1.Event { return eventProto("", event) })}, nil
}

func (s *ControlService) WatchEvents(ctx context.Context, req *controlv1.WatchEventsRequest, stream *connect.ServerStream[controlv1.Event]) error {
	if req == nil {
		return rpcError(control.Errorf(control.ErrorInvalidInput, "request is required"))
	}
	if s.events == nil {
		return rpcError(control.Errorf(control.ErrorInvalidInput, "event store is not configured"))
	}
	events, backlog, unsubscribe := s.events.SubscribeAndList()
	defer unsubscribe()
	for _, event := range mapProto(eventsAfter(backlog, req.GetAfterId()), func(event control.Event) *controlv1.Event { return eventProto("", event) }) {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(eventProto("", event)); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
