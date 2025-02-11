package tracing

import (
	"context"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type traceID struct{}

func InjectTraceID(ctx context.Context) context.Context {
	id := uuid.New().String()
	logger := log.With().Str("traceId", id).Logger()
	return logger.WithContext(ctx)
}
