package consumer

import (
	"context"

	"github.com/babylonlabs-io/staking-queue-client/client"
)

//go:generate mockery --name=EventConsumer --output=../tests/mocks --outpkg=mocks --filename=mock_event_consumer.go
type EventConsumer interface {
	Start() error
	PushActiveStakingEvent(ctx context.Context, ev *client.StakingEvent) error
	PushUnbondingStakingEvent(ctx context.Context, ev *client.StakingEvent) error
	Stop() error
}
