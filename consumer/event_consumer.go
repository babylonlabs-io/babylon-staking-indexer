package consumer

import (
	"github.com/babylonlabs-io/staking-queue-client/client"
)

type EventConsumer interface {
	Start() error
	PushActiveStakingEvent(ev *client.StakingEvent) error
	PushUnbondingStakingEvent(ev *client.StakingEvent) error
	PushWithdrawableStakingEvent(ev *client.StakingEvent) error
	PushWithdrawnStakingEvent(ev *client.StakingEvent) error
	PushSlashedStakingEvent(ev *client.StakingEvent) error
	Stop() error
}
