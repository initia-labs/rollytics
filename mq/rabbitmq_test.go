package mq

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/initia-labs/rollytics/config"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SkipTest is a utility function that skips tests if the environment variable "CI" or "GITHUB_ACTIONS" is set.
// This is useful for tests that require a local RabbitMQ instance and should not run in a CI/CD pipeline.
func SkipTest(t *testing.T) {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test in CI environment")
	}
}

// getTestRabbitMQConfig provides a standard RabbitMQ configuration for testing purposes.
// It points to a local RabbitMQ instance with default credentials.
func getTestRabbitMQConfig() config.RabbitMQConfig {
	return config.RabbitMQConfig{
		Host:       "localhost",
		Port:       5552,
		VHost:      "rollytics",
		User:       "admin",
		Password:   "admin",
		Partitions: 3,
	}
}

// waitOrTimeout is a helper function that waits for a sync.WaitGroup to complete or times out after a fixed duration.
// This is crucial for asynchronous tests to prevent them from hanging indefinitely.
//
// Parameters:
//   - t: The testing.T instance.
//   - wg: The WaitGroup to wait on.
//   - name: A descriptive name for the test, used in log messages.
func waitOrTimeout(t *testing.T, wg *sync.WaitGroup, name string) {
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		log.Printf("✅[%s] success", name)
	case <-time.After(10 * time.Second):
		t.Errorf("❌Timeout in [%s] test", name)
	}
}

// TestRabbitMQ serves as the main entry point for the RabbitMQ test suite.
// It orchestrates the execution of all major tests, ensuring they are run in a specific order.
func TestRabbitMQ(t *testing.T) {
	SkipTest(t)
	TestBasicDelivery(t)
	TestReplayability(t)
	TestParallelConsumers(t)
}

// TestBasicDelivery verifies the fundamental functionality of the message queue.
// It tests the entire lifecycle: creating a stream, publishing messages to it,
// and consuming them from the beginning.
func TestBasicDelivery(t *testing.T) {
	// 1. Setup: Get RabbitMQ config and define names for stream and consumer.
	rmqConfig := getTestRabbitMQConfig()
	chainID := "doi-basic-test"
	consumerName := "doi-basic"

	// 2. Producer: Create a new producer and ensure it's closed after the test.
	producer, err := NewProducer(rmqConfig)
	require.NoError(t, err)
	defer producer.Close()

	// 3. Stream Management: Clean up any old stream and declare a new one.
	_ = producer.DeleteStream(chainID)
	require.NoError(t, producer.DeclareStream(chainID, rmqConfig.Partitions))

	// 4. Consumer: Create a consumer to listen for messages.
	msgCount := 3
	var wg sync.WaitGroup

	consumer, err := NewConsumer(rmqConfig, chainID, consumerName)
	require.NoError(t, err)
	defer consumer.Close()

	// 5. Subscription: Start a goroutine to subscribe to the stream from the 'first' message.
	wg.Add(1)
	go func() {
		received := 0
		err := consumer.Subscribe("first", consumerName, func(msg *amqp.Message) {
			var m Message
			_ = json.Unmarshal(msg.GetData(), &m)
			log.Printf("[Consumer %s] height=%d hash=%v", consumerName, m.Height, m.Hash)
			received++
			if received == msgCount {
				wg.Done() // Signal that all messages have been received.
			}
		})
		require.NoError(t, err)
	}()

	time.Sleep(1 * time.Second) // Give the consumer a moment to start up.

	// 6. Publishing: Send a few messages to the stream.
	for i := 1; i <= msgCount; i++ {
		err := producer.Publish(chainID, int64(i), []string{fmt.Sprintf("hash%d", i)}, rmqConfig.Partitions)
		require.NoError(t, err)
	}

	// 7. Verification: Wait for the consumer to receive all messages or time out.
	waitOrTimeout(t, &wg, "TestBasicDelivery")
}

// TestReplayability checks if the consumer can correctly replay messages from different starting points.
// It covers replaying from the 'first' message, a specific height, and a later height.
func TestReplayability(t *testing.T) {
	rmqConfig := getTestRabbitMQConfig()
	chainID := "doi-replayability-test"

	producer, err := NewProducer(rmqConfig)
	require.NoError(t, err)
	defer producer.Close()

	_ = producer.DeleteStream(chainID)
	require.NoError(t, producer.DeclareStream(chainID, rmqConfig.Partitions))

	// 1. Publishing: Send a set of messages to be replayed.
	const msgCount = 5
	var sentMessages []Message
	for i := 0; i < msgCount; i++ {
		height := int64(i)
		hash := []string{fmt.Sprintf("hash%d", i)}
		sentMessages = append(sentMessages, Message{Height: height, Hash: hash})
		err := producer.Publish(chainID, height, hash, rmqConfig.Partitions)
		require.NoError(t, err)
	}

	time.Sleep(1 * time.Second)

	// 2. Test Cases: Define scenarios for replaying messages.
	testCases := []struct {
		name             string
		subscriptionType string
		expectedMessages []Message
	}{
		{"from_first", "first", sentMessages},
		{"from_height_0", "height:0", sentMessages},
		{"from_height_2", "height:2", sentMessages[2:]},
	}

	// 3. Execution: Run each test case in a sub-test.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			consumerName := fmt.Sprintf("doi-replay-%s", tc.name)
			consumer, err := NewConsumer(rmqConfig, chainID, consumerName)
			require.NoError(t, err)
			defer consumer.Close()

			var (
				mtx      sync.Mutex
				received []Message
				wg       sync.WaitGroup
			)

			// 4. Subscription: Subscribe with the specified replay strategy.
			wg.Add(1)
			go func() {
				err := consumer.Subscribe(tc.subscriptionType, consumerName, func(msg *amqp.Message) {
					var m Message
					_ = json.Unmarshal(msg.GetData(), &m)
					mtx.Lock()
					defer mtx.Unlock()
					received = append(received, m)
					if len(received) == len(tc.expectedMessages) {
						wg.Done()
					}
				})
				if err != nil {
					t.Errorf("Subscribe error: %v", err)
				}
			}()

			// 5. Verification: Wait for messages and assert they match expectations.
			waitOrTimeout(t, &wg, tc.name)

			sort.Slice(received, func(i, j int) bool { return received[i].Height < received[j].Height })
			assert.Equal(t, tc.expectedMessages, received)
		})
	}
}

// TestParallelConsumers ensures that when multiple consumers share the same name (group),
// they correctly coordinate to process messages. RabbitMQ's Single Active Consumer (SAC)
// feature should ensure that only one consumer is active per partition at any given time.
func TestParallelConsumers(t *testing.T) {
	rmqConfig := getTestRabbitMQConfig()
	chainID := "doi-parallel-test"
	consumerName := "doi-parallel"
	msgCount := 4

	producer, err := NewProducer(rmqConfig)
	require.NoError(t, err)
	defer producer.Close()

	_ = producer.DeleteStream(chainID)
	require.NoError(t, producer.DeclareStream(chainID, rmqConfig.Partitions))

	var wg sync.WaitGroup
	wg.Add(1)
	var mu sync.Mutex
	totalReceived := 0

	// 1. Consumer Setup: Helper to start a consumer with a given label.
	startConsumer := func(label string) {
		consumer, err := NewConsumer(rmqConfig, chainID, consumerName)
		require.NoError(t, err)
		defer consumer.Close()

		go consumer.Subscribe("last", consumerName, func(msg *amqp.Message) {
			var m Message
			_ = json.Unmarshal(msg.GetData(), &m)
			log.Printf("[%s] height=%d hash=%v", label, m.Height, m.Hash)
			mu.Lock()
			defer mu.Unlock()
			totalReceived++
			if totalReceived == msgCount {
				wg.Done()
			}
		})
	}

	// 2. Start Consumers: Launch two consumers with the same name but different labels.
	startConsumer("Consumer Moro")
	startConsumer("Consumer Rene")

	time.Sleep(1 * time.Second)

	// 3. Publishing: Send messages to the stream.
	for i := 1; i <= msgCount; i++ {
		err := producer.Publish(chainID, int64(i), []string{fmt.Sprintf("hash%d", i)}, rmqConfig.Partitions)
		if err != nil {
			t.Errorf("publish failed: %v", err)
		}
	}

	// 4. Verification: Wait for all messages to be processed by the consumer group.
	waitOrTimeout(t, &wg, "TestParallelConsumers")
}
