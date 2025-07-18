package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/initia-labs/rollytics/config"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

// Consumer is a reliable RabbitMQ stream consumer.
type Consumer struct {
	env          *stream.Environment
	chainID      string
	consumerName string
	consumer     *stream.SuperStreamConsumer
	mu           sync.Mutex
}

// NewConsumer creates a new reliable consumer for a given stream and consumer name using the provided RabbitMQConfig.
func NewConsumer(cfg config.RabbitMQConfig, chainID string, consumerName string) (*Consumer, error) {
	env, err := stream.NewEnvironment(stream.NewEnvironmentOptions().
		SetHost(cfg.Host).
		SetPort(cfg.Port).
		SetVHost(cfg.VHost).
		SetUser(cfg.User).
		SetPassword(cfg.Password))
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}
	return &Consumer{
		env:          env,
		chainID:      chainID,
		consumerName: consumerName,
	}, nil
}

// Subscribe subscribes to a partitioned stream (was super stream) with partitioned consumption.
// consumerName is the group name for SAC (single active consumer) or partitioned group.
// handler is called for each message.
//
// This function supports the following `subscriptionType` values:
//   - "first": Starts consuming from the very first message in the stream.
//   - "last": Starts consuming from the latest message in the stream.
//   - "height:<number>": Implements client-side filtering to support replaying from a specific block height.
//     The consumer subscribes from the beginning of the stream ("first") and then discards messages
//     until it finds a message with a `Height` field greater than or equal to the specified number.
func (c *Consumer) Subscribe(subscriptionType string, consumerName string, handler func(message *amqp.Message)) error {
	var offsetSpec stream.OffsetSpecification
	var startHeight int64 = -1

	if subscriptionType == "first" {
		offsetSpec = stream.OffsetSpecification{}.First()
	} else if subscriptionType == "last" {
		offsetSpec = stream.OffsetSpecification{}.Last()
	} else if strings.HasPrefix(subscriptionType, "height:") {
		parts := strings.Split(subscriptionType, ":")
		if len(parts) != 2 {
			return errors.New("invalid height format; expected 'height:<number>'")
		}
		height, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid height value: %w", err)
		}
		// Set the start height for client-side filtering.
		startHeight = height
		// Always start from the beginning of the stream to be able to filter from the desired height.
		offsetSpec = stream.OffsetSpecification{}.First()
	} else {
		return errors.New("unknown subscriptionType")
	}

	handleMessages := func(consumerContext stream.ConsumerContext, message *amqp.Message) {
		// If a startHeight is specified, filter messages until the height is reached.
		if startHeight >= 0 {
			var msg Message
			if err := json.Unmarshal(message.GetData(), &msg); err != nil {
				log.Printf("error unmarshalling message: %v", err)
				return // Don't process malformed messages.
			}
			// Discard messages with a height lower than the desired start height.
			if msg.Height < startHeight {
				return // Skip message
			}
		}
		handler(message)
	}

	sac := stream.NewSingleActiveConsumer(
		func(partition string, isActive bool) stream.OffsetSpecification {
			restart := offsetSpec
			return restart
		},
	)

	consumer, err := c.env.NewSuperStreamConsumer(
		c.chainID,
		handleMessages,
		stream.NewSuperStreamConsumerOptions().
			SetSingleActiveConsumer(sac.SetEnabled(true)).
			SetConsumerName(consumerName).
			SetOffset(offsetSpec),
	)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	c.consumer = consumer
	log.Printf("[Consumer %s] Subscribed to stream %s from %s", consumerName, c.chainID, subscriptionType)
	return nil
}

// Close closes the consumer and the environment.
func (c *Consumer) Close() error {
	var firstErr error
	if c.consumer != nil {
		err := c.consumer.Close()
		if err != nil {
			firstErr = err
		}
	}
	if c.env != nil {
		err := c.env.Close()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
