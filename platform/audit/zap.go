package audit

import (
	"context"

	"llamarig/core/control"

	"go.uber.org/zap"
)

type Sink struct {
	logger *zap.Logger
}

func NewSink(logger *zap.Logger) Sink {
	if logger == nil {
		logger = zap.NewNop()
	}
	return Sink{logger: logger}
}

func (s Sink) Record(_ context.Context, event control.AuditEvent) {
	fields := []zap.Field{
		zap.String("action", event.Action),
		zap.Bool("success", event.Success),
		zap.Duration("duration", event.Duration),
	}
	if event.ErrorKind != "" {
		fields = append(fields, zap.String("error_kind", string(event.ErrorKind)))
	}
	s.logger.Info("audit event", fields...)
}
