package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/babylonlabs-io/staking-queue-client/config"
)

const (
	dlxName             = "common_dlx"
	dlxRoutingPostfix   = "_routing_key"
	delayedQueuePostfix = "_delay"
)

type RabbitMqClient struct {
	connection         *amqp.Connection
	channel            *amqp.Channel
	queueName          string
	stopCh             chan struct{} // This is used to gracefully stop the message receiving loop
	delayedRequeueTime time.Duration
}

func NewRabbitMqClient(config *config.QueueConfig, queueName string) (*RabbitMqClient, error) {
	amqpURI := fmt.Sprintf("amqp://%s:%s@%s", config.QueueUser, config.QueuePassword, config.Url)

	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	// Declare a single common DLX for all queues
	err = ch.ExchangeDeclare(dlxName,
		"direct",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-queue-type": config.QueueType,
		},
	)
	if err != nil {
		return nil, err
	}

	// Declare a delay queue specific to this particular queue
	delayQueueName := queueName + delayedQueuePostfix
	_, err = ch.QueueDeclare(
		delayQueueName,
		true,
		false,
		false,
		false,
		amqp.Table{
			// Default exchange to route messages back to the main queue
			// The "" in rabbitMq referring to the default exchange which allows
			// to route messages to the queue by the routing key which is the queue name
			"x-queue-type":              config.QueueType,
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": queueName,
		},
	)
	if err != nil {
		return nil, err
	}

	// Declare the queue that will be created if not exists
	customDlxRoutingKey := queueName + dlxRoutingPostfix
	_, err = ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		amqp.Table{
			"x-queue-type":              config.QueueType,
			"x-dead-letter-exchange":    dlxName,
			"x-dead-letter-routing-key": customDlxRoutingKey,
		},
	)
	if err != nil {
		return nil, err
	}

	// Bind the delay queue to the common DLX
	err = ch.QueueBind(delayQueueName, customDlxRoutingKey, dlxName, false, nil)
	if err != nil {
		return nil, err
	}

	err = ch.Confirm(false)
	if err != nil {
		return nil, err
	}

	return &RabbitMqClient{
		connection:         conn,
		channel:            ch,
		queueName:          queueName,
		stopCh:             make(chan struct{}),
		delayedRequeueTime: time.Duration(config.ReQueueDelayTime) * time.Second,
	}, nil
}

// Ping checks the health of the RabbitMQ infrastructure.
func (c *RabbitMqClient) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Check if the RabbitMQ connection is closed
		if c.connection.IsClosed() {
			return fmt.Errorf("rabbitMQ connection is closed")
		}

		// Check if the RabbitMQ channel is closed
		if c.channel.IsClosed() {
			return fmt.Errorf("rabbitMQ channel is closed")
		}
	}

	return nil
}

func (c *RabbitMqClient) ReceiveMessages() (<-chan QueueMessage, error) {
	msgs, err := c.channel.Consume(
		c.queueName, // queueName
		"",          // consumer
		false,       // auto-ack. We want to manually acknowledge the message after processing it.
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return nil, err
	}
	output := make(chan QueueMessage)
	go func() {
		defer close(output)
		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					return // Channel closed, exit goroutine
				}
				attempts := d.Headers["x-processing-attempts"]
				if attempts == nil {
					attempts = int32(0)
				}
				currentAttempt := attempts.(int32)

				output <- QueueMessage{
					Body:          string(d.Body),
					Receipt:       strconv.FormatUint(d.DeliveryTag, 10),
					RetryAttempts: currentAttempt,
				}
			case <-c.stopCh:
				return // Stop signal received, exit goroutine
			}
		}
	}()

	return output, nil
}

// DeleteMessage deletes a message from the queue. In RabbitMQ, this is equivalent to acknowledging the message.
// The deliveryTag is the unique identifier for the message.
func (c *RabbitMqClient) DeleteMessage(deliveryTag string) error {
	deliveryTagInt, err := strconv.ParseUint(deliveryTag, 10, 64)
	if err != nil {
		return err
	}
	return c.channel.Ack(deliveryTagInt, false)
}

// ReQueueMessage requeues a message back to the queue with a delay.
// This is done by sending the message again with an incremented counter.
// The original message is then deleted from the queue.
func (c *RabbitMqClient) ReQueueMessage(ctx context.Context, message QueueMessage) error {
	// For requeueing, we will send the message to a delay queue that has a TTL pre-configured.
	delayQueueName := c.queueName + delayedQueuePostfix
	err := c.sendMessageWithAttempts(ctx, message.Body, delayQueueName, message.IncrementRetryAttempts(), c.delayedRequeueTime)
	if err != nil {
		return fmt.Errorf("failed to requeue message: %w", err)
	}

	err = c.DeleteMessage(message.Receipt)
	if err != nil {
		return fmt.Errorf("failed to delete message while requeuing: %w", err)
	}

	return nil
}

// SendMessage sends a message to the queue. the ctx is used to control the timeout of the operation.
func (c *RabbitMqClient) sendMessageWithAttempts(ctx context.Context, messageBody, queueName string, attempts int32, ttl time.Duration) error {
	// Ensure the channel is open
	if c.channel == nil {
		return fmt.Errorf("RabbitMQ channel not initialized")
	}

	// Prepare new headers with the incremented counter
	newHeaders := amqp.Table{
		"x-processing-attempts": attempts,
	}

	publishMsg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "text/plain",
		Body:         []byte(messageBody),
		Headers:      newHeaders,
	}

	// Exclude the expiration if the TTL is 0.
	if ttl > 0 {
		publishMsg.Expiration = strconv.Itoa(int(ttl.Milliseconds()))
	}

	// Publish a message to the queue
	confirmation, err := c.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"",        // exchange: Use the default exchange
		queueName, // routing key: The queue this message should be routed to
		true,      // mandatory: true indicates the server must route the message to a queue, otherwise error
		false,     // immediate: false indicates the server may wait to send the message until a consumer is available
		publishMsg,
	)

	if err != nil {
		return fmt.Errorf("failed to publish a message to queue %s: %w", queueName, err)
	}

	if confirmation == nil {
		return fmt.Errorf("message not confirmed when publishing into queue %s", queueName)
	}
	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to confirm message when publishing into queue %s: %w", queueName, err)
	}
	if !confirmed {
		return fmt.Errorf("message not confirmed when publishing into queue %s", queueName)
	}

	return nil
}

// SendMessage sends a message to the queue. the ctx is used to control the timeout of the operation.
func (c *RabbitMqClient) SendMessage(ctx context.Context, messageBody string) error {
	return c.sendMessageWithAttempts(ctx, messageBody, c.queueName, 0, 0)
}

// Stop stops the message receiving process.
func (c *RabbitMqClient) Stop() error {
	if err := c.channel.Close(); err != nil {
		return err
	}
	if err := c.connection.Close(); err != nil {
		return err
	}

	close(c.stopCh)

	return nil
}

func (c *RabbitMqClient) GetQueueName() string {
	return c.queueName
}
