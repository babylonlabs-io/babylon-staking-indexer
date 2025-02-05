package poller

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

type Poller struct {
	interval   time.Duration
	quit       chan struct{}
	pollMethod func(ctx context.Context) error
	logger     zerolog.Logger
}

func NewPoller(interval time.Duration, logger zerolog.Logger, pollMethod func(ctx context.Context) error) *Poller {
	return &Poller{
		interval:   interval,
		quit:       make(chan struct{}),
		pollMethod: pollMethod,
		logger:     logger,
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)

	for {
		select {
		case <-ticker.C:
			if err := p.pollMethod(ctx); err != nil {
				p.logger.Error().Err(err).Msg("Error polling")
			}
		case <-ctx.Done():
			// Handle context cancellation.
			p.logger.Info().Msg("Poller stopped due to context cancellation")
			return
		case <-p.quit:
			p.logger.Info().Msg("Poller stopped")
			ticker.Stop() // Stop the ticker
			return
		}
	}
}

func (p *Poller) Stop() {
	close(p.quit)
}
