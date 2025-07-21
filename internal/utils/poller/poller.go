package poller

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/tracing"

	"github.com/rs/zerolog/log"
)

type Poller struct {
	interval   time.Duration
	pollMethod func(ctx context.Context) error
}

func NewPoller(interval time.Duration, pollMethod func(ctx context.Context) error) *Poller {
	return &Poller{
		interval:   interval,
		pollMethod: pollMethod,
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	log := log.Ctx(ctx)
	for {
		select {
		case <-ticker.C:
			ctx = tracing.InjectTraceID(ctx)
			if err := p.pollMethod(ctx); err != nil {
				log.Error().Err(err).Msg("Error polling")
			}
		case <-ctx.Done():
			// Handle context cancellation.
			log.Info().Msg("Poller stopped due to context cancellation")
			return
		}
	}
}
