package e2etest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/babylonlabs-io/staking-queue-client/queuemngr"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonlabs-io/staking-queue-client/config"
)

func setupTestQueueConsumer(t *testing.T, cfg *config.QueueConfig) (*queuemngr.QueueManager, error) {
	amqpURI := fmt.Sprintf("amqp://%s:%s@%s", cfg.QueueUser, cfg.QueuePassword, cfg.Url)
	conn, err := amqp091.Dial(amqpURI)
	if err != nil {
		t.Fatalf("failed to connect to RabbitMQ in test: %v", err)
	}
	defer conn.Close()
	err = purgeQueues(conn, []string{
		client.ActiveStakingQueueName,
	})
	if err != nil {
		return nil, err
	}

	// Start the actual queue processing in our codebase

	err = cfg.Validate()
	require.NoError(t, err)
	queues, err := queuemngr.NewQueueManager(cfg, zap.NewNop())
	require.NoError(t, err)

	return queues, nil
}

// purgeQueues purges all messages from the given list of queues.
func purgeQueues(conn *amqp091.Connection, queues []string) error {
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel in test: %w", err)
	}
	defer ch.Close()

	for _, queue := range queues {
		_, err := ch.QueuePurge(queue, false)
		if err != nil {
			if strings.Contains(err.Error(), "no queue") {
				fmt.Printf("Queue '%s' not found, ignoring...\n", queue)
				continue // Ignore this error and proceed with the next queue
			}
			return fmt.Errorf("failed to purge queue in test %s: %w", queue, err)
		}
	}

	return nil
}
