package consumer

import (
	"github.com/babylonlabs-io/staking-queue-client/client"
)

type EventConsumer interface {
	Start() error
	PushActiveEventV2(ev *client.StakingEvent) error
	PushUnbondingEventV2(ev *client.StakingEvent) error
	Stop() error
}
