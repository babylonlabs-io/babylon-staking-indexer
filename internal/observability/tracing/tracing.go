package tracing

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type TracingContextKey string

const TracingInfoKey = TracingContextKey("requestTracingInfo")
const TraceIdKey = TracingContextKey("requestTraceId")

type SpanDetail struct {
	Name     string
	Duration int64
}

type TracingInfo struct {
	SpanDetails []SpanDetail
}

func (t *TracingInfo) addSpanDetail(detail SpanDetail) {
	t.SpanDetails = append(t.SpanDetails, detail)
}

func WrapWithSpan[Result any](ctx context.Context, name string, next func() (Result, error)) (Result, error) {
	tracingInfo, ok := ctx.Value(TracingInfoKey).(*TracingInfo)
	if !ok {
		log.Error().Msg("TracingInfo not found in the request chain")
	}

	startTime := time.Now()
	defer func() {
		if tracingInfo != nil {
			duration := time.Since(startTime).Milliseconds()
			tracingInfo.addSpanDetail(SpanDetail{Name: name, Duration: duration})
		}
	}()

	return next()
}
