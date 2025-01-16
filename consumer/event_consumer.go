package consumer

import (
	"context"

	"github.com/babylonlabs-io/staking-queue-client/client"
)

type EventConsumer interface {
	Start() error
	PushActiveStakingEvent(ctx context.Context, ev *client.StakingEvent) error
	PushUnbondingStakingEvent(ctx context.Context, ev *client.StakingEvent) error
	PushWithdrawableStakingEvent(ctx context.Context, ev *client.StakingEvent) error
	PushWithdrawnStakingEvent(ctx context.Context, ev *client.StakingEvent) error
	Stop() error
}
