package consumer

import (
	"github.com/babylonlabs-io/staking-queue-client/client"
)

type EventConsumer interface {
	Start() error
	PushStakingEvent(ev *client.ActiveStakingEvent) error
	PushUnbondingEvent(ev *client.UnbondingStakingEvent) error
	PushWithdrawEvent(ev *client.WithdrawStakingEvent) error
	PushExpiryEvent(ev *client.ExpiredStakingEvent) error
	PushBtcInfoEvent(ev *client.BtcInfoEvent) error
	PushConfirmedInfoEvent(ev *client.ConfirmedInfoEvent) error
	Stop() error
}
