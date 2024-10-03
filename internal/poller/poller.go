package poller

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
)

type Poller struct {
	service  *services.Service
	interval time.Duration
	quit     chan struct{}
}

func NewPoller(interval time.Duration, service *services.Service) (*Poller, error) {
	return &Poller{
		service:  service,
		interval: interval,
		quit:     make(chan struct{}),
	}, nil
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)

	for {
		select {
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				log.Error().Err(err).Msg("Error polling")
			}
		case <-ctx.Done():
			// Handle context cancellation.
			log.Info().Msg("Poller stopped due to context cancellation")
			return
		case <-p.quit:
			ticker.Stop() // Stop the ticker
			return
		}
	}
}

func (p *Poller) Stop() {
	close(p.quit)
}

func (p *Poller) poll(ctx context.Context) error {
	return nil
}
