package consumer

import (
	"github.com/babylonlabs-io/staking-queue-client/client"
)

type EventConsumer interface {
	Start() error
	PushStakingEvent(ev *client.StakingEvent) error
	PushUnbondingEvent(ev *client.StakingEvent) error
	Stop() error
}
