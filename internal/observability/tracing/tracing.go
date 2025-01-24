package tracing

import (
	"context"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type TraceID struct{}

func ContextWithTraceID(ctx context.Context) context.Context {
	traceID := uuid.New().String()
	return context.WithValue(ctx, TraceID{}, traceID)
}

func LogWithTraceID(ctx context.Context, log zerolog.Logger) zerolog.Logger {
	traceID := ctx.Value(TraceID{})
	if traceID != nil {
		return log
	}

	return log.With().Str("traceID", traceID.(string)).Logger()
}
