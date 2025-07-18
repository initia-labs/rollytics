# RabbitMQ Stream Messaging (`mq`)

This package provides a high-level Go wrapper for interacting with RabbitMQ streams. It simplifies the implementation of reliable producers and consumers, making it easy to integrate high-throughput messaging into your applications.

## Features

*   **Simplified Producer**: An easy-to-use producer for publishing messages to a super stream.
*   **Simplified Consumer**: A straightforward consumer for subscribing to a stream with automatic offset management.
*   **Super Streams**: Built-in support for RabbitMQ's super streams for partitioned, scalable message queues.
*   **Message Replayability**: Consumers can start from the beginning of a stream (`first`), the end (`last`), or a specific message offset (e.g., `height:123`).
*   **Consumer Groups**: Supports consumer groups (Single Active Consumer) out of the box, allowing multiple consumer instances to coordinate message processing.

## Prerequisites

Before using this package, you need a running RabbitMQ instance with the `stream` plugin enabled. The tests and examples are configured to connect to a local instance at `localhost:5552`.

## Usage

### Configuration

First, you need to define your RabbitMQ configuration.

```go
import "github.com/initia-labs/rollytics/config"

rmqConfig := config.RabbitMQConfig{
    Host:       "localhost",
    Port:       5552,
    VHost:      "rollytics",
    User:       "admin",
    Password:   "admin",
    Partitions: 3, // Number of partitions for the super stream
}
```

### Producer Example

The `Producer` is used to send messages to a stream.

```go
package main

import (
	"fmt"
	"log"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/mq"
)

func main() {
	// 1. Get RabbitMQ config
	rmqConfig := config.RabbitMQConfig{
		Host:       "localhost",
		Port:       5552,
		VHost:      "rollytics",
		User:       "admin",
		Password:   "admin",
		Partitions: 3,
	}
	chainID := "my-chain-stream"

	// 2. Create a new producer
	producer, err := mq.NewProducer(rmqConfig)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()

	// 3. Declare the stream (optional, Publish will do it automatically)
	// It's good practice to declare it explicitly if you need to manage it.
	if err := producer.DeclareStream(chainID, rmqConfig.Partitions); err != nil {
		log.Fatalf("Failed to declare stream: %v", err)
	}

	// 4. Publish a message
	height := int64(1)
	hash := []string{"0xabc123"}
	if err := producer.Publish(chainID, height, hash, rmqConfig.Partitions); err != nil {
		log.Fatalf("Failed to publish message: %v", err)
	}

	fmt.Println("Message published successfully!")
}
```

### Consumer Example

The `Consumer` subscribes to a stream and processes messages.

```go
package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/mq"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
)

func main() {
	// 1. Get RabbitMQ config
	rmqConfig := config.RabbitMQConfig{
		Host:       "localhost",
		Port:       5552,
		VHost:      "rollytics",
		User:       "admin",
		Password:   "admin",
		Partitions: 3,
	}
	chainID := "my-chain-stream"
	consumerName := "my-consumer"

	// 2. Create a new consumer
	consumer, err := mq.NewConsumer(rmqConfig, chainID, consumerName)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	// 3. Subscribe to the stream
	// Subscription types: "first", "last", "height:N"
	subscriptionType := "first"
	err = consumer.Subscribe(subscriptionType, consumerName, func(msg *amqp.Message) {
		var m mq.Message
		if err := json.Unmarshal(msg.GetData(), &m); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return
		}
		log.Printf("Received message: Height=%d, Hash=%v", m.Height, m.Hash)
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("Consumer is running. Press CTRL+C to exit.")

	// 4. Wait for a termination signal to gracefully shut down
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
```

## Running Tests

The tests in `rabbitmq_test.go` provide comprehensive examples of the package's functionality. To run them, you need a local RabbitMQ instance running.

```sh
go test -v ./mq
```

> **Note**: The tests will be skipped if a `CI` or `GITHUB_ACTIONS` environment variable is detected.
