package poller

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog/log"
)

type Poller struct {
	interval   time.Duration
	quit       chan struct{}
	pollMethod func(ctx context.Context) *types.Error
}

func NewPoller(interval time.Duration, pollMethod func(ctx context.Context) *types.Error) *Poller {
	return &Poller{
		interval:   interval,
		quit:       make(chan struct{}),
		pollMethod: pollMethod,
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	log.Info().Msgf("Starting poller with interval %s", p.interval)

	for {
		select {
		case <-ticker.C:
			log.Debug().Msg("Executing poll method")
			if err := p.pollMethod(ctx); err != nil {
				log.Error().Err(err).Msg("Error polling")
			} else {
				log.Debug().Msg("Poll method executed successfully")
			}
		case <-ctx.Done():
			log.Info().Msg("Poller stopped due to context cancellation")
			return
		case <-p.quit:
			log.Info().Msg("Poller stopped")
			return
		}
	}
}

func (p *Poller) Stop() {
	close(p.quit)
}
