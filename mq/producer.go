package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/initia-labs/rollytics/config"

	"github.com/google/uuid"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/message"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

// Producer manages the connection and publishing of messages to RabbitMQ streams.
// It holds a stream environment and a map of producers for different chains,
// ensuring that messages are sent to the correct stream. The struct uses a sync.Map
// for concurrent access to the producers.
type Producer struct {
	env       *stream.Environment
	producers sync.Map // map[chainID]*stream.SuperStreamProducer
}

// NewProducer initializes a new Producer with the given RabbitMQ configuration.
// It sets up the stream environment, which is the foundation for creating streams and producers.
//
// Parameters:
//   - cfg: RabbitMQ configuration containing connection details like host, port, user, and password.
//
// Returns:
//   - A pointer to the new Producer or an error if the environment setup fails.
func NewProducer(cfg config.RabbitMQConfig) (*Producer, error) {
	env, err := stream.NewEnvironment(stream.NewEnvironmentOptions().
		SetHost(cfg.Host).
		SetPort(cfg.Port).
		SetVHost(cfg.VHost).
		SetUser(cfg.User).
		SetPassword(cfg.Password))
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}
return &Producer{env: env}, nil
}

// DeclareStream creates a super stream for a given chainID with a specified number of partitions.
// Super streams are a RabbitMQ feature that allows for partitioned, high-throughput message queues.
// If the stream already exists, this function does nothing.
//
// Parameters:
//   - chainID: The identifier for the chain, used as the stream name.
//   - partitions: The number of partitions for the super stream. If less than 1, it defaults to 1.
//
// Returns:
//   - An error if the stream declaration fails for reasons other than the stream already existing.
func (p *Producer) DeclareStream(chainID string, partitions int) error {
	if partitions < 1 {
		partitions = 1
	}
	// Create a super stream
	err := p.env.DeclareSuperStream(chainID,
		stream.NewPartitionsOptions(partitions).
			SetMaxLengthBytes(stream.ByteCapacity{}.GB(2)))
	if err != nil && !errors.Is(err, stream.StreamAlreadyExists) {
		return err
	}
	return nil
}

// DeleteStream removes a super stream associated with the given chainID.
// This is useful for cleaning up resources, especially in test environments.
//
// Parameters:
//   - chainID: The identifier for the chain whose stream should be deleted.
//
// Returns:
//   - An error if the stream deletion fails.
func (p *Producer) DeleteStream(chainID string) error {
	err := p.env.DeleteSuperStream(chainID)
	if err != nil {
		return fmt.Errorf("failed to delete stream: %w", err)
	}
	return nil
}

// Publish sends a message to the specified stream.
// It ensures the stream exists, creates a producer if one doesn't already exist for the chainID,
// and then sends the message. The message is routed to a partition based on a hash of its message ID,
// ensuring even distribution.
//
// Parameters:
//   - chainID: The identifier for the chain, used as the stream name.
//   - height: The block height associated with the message.
//   - hash: A slice of transaction hashes included in the block.
//   - partitions: The number of partitions for the stream.
//
// Returns:
//   - An error if declaring the stream, creating a producer, marshalling the message, or sending the message fails.
func (p *Producer) Publish(chainID string, height int64, hash []string, partitions int) error {
	if err := p.DeclareStream(chainID, partitions); err != nil {
		return fmt.Errorf("failed to declare stream: %w", err)
	}
	// Use or create a producer for this stream
	val, ok := p.producers.Load(chainID)
	var prod *stream.SuperStreamProducer
	if ok {
		prod = val.(*stream.SuperStreamProducer)
	} else {
		var prodErr error
		prod, prodErr = p.env.NewSuperStreamProducer(chainID,
			stream.NewSuperStreamProducerOptions(
				stream.NewHashRoutingStrategy(func(msg message.StreamMessage) string {
					return msg.GetMessageProperties().MessageID.(string)
				}),
			),
		)
		if prodErr != nil {
			return fmt.Errorf("failed to create producer: %w", prodErr)
		}
		p.producers.Store(chainID, prod)
	}

	// Prepare message
	meta := Message{Height: height, Hash: hash}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal Message: %w", err)
	}
	msg := amqp.NewMessage(data)
	msg.Properties = &amqp.MessageProperties{
		MessageID: uuid.New().String(),
	}
	return prod.Send(msg)
}

// Close gracefully shuts down all active producers and the stream environment.
// It iterates through all stored producers, closes them, and then closes the environment.
// This ensures that all resources are properly released.
//
// Returns:
//   - The first error encountered during the closing of producers or the environment.
func (p *Producer) Close() error {
	var firstErr error
	p.producers.Range(func(key, value any) bool {
		if prod, ok := value.(*stream.SuperStreamProducer); ok && prod != nil {
			err := prod.Close()
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return true
	})
	if p.env != nil {
		err := p.env.Close()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
