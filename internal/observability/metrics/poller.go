package metrics

import (
	"context"
	"time"
)

// pollerFunction alias is private and should be used only here
type pollerFunction = func(ctx context.Context) error

func RecordPollerDuration(typ string, f pollerFunction) pollerFunction {
	return func(ctx context.Context) error {
		startTime := time.Now()
		err := f(ctx)
		duration := time.Since(startTime).Seconds()

		status := Success
		if err != nil {
			status = Error
		}
		pollerDurationHistogram.WithLabelValues(typ, status.String()).Observe(duration)

		return err
	}
}
