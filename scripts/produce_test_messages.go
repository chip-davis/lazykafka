package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run produce_test_messages.go <bootstrap-servers> <topic> <num-messages>")
		fmt.Println("Example: go run produce_test_messages.go localhost:9092 test-topic 5000")
		os.Exit(1)
	}

	bootstrapServers := os.Args[1]
	topic := os.Args[2]
	numMessages, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Printf("Invalid number of messages: %v\n", err)
		os.Exit(1)
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrapServers),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Printf("Producing %d messages to topic '%s'...\n", numMessages, topic)
	start := time.Now()

	ctx := context.Background()
	for i := 0; i < numMessages; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf(`{"id": %d, "timestamp": "%s", "message": "This is test message number %d with some additional data to make it more realistic. Lorem ipsum dolor sit amet, consectetur adipiscing elit."}`,
			i,
			time.Now().Format(time.RFC3339),
			i)

		record := &kgo.Record{
			Key:   []byte(key),
			Value: []byte(value),
			Topic: topic,
		}

		client.Produce(ctx, record, func(r *kgo.Record, err error) {
			if err != nil {
				fmt.Printf("Error producing message %d: %v\n", i, err)
			}
		})

		if (i+1)%1000 == 0 {
			fmt.Printf("  Produced %d messages...\n", i+1)
		}
	}

	fmt.Println("Flushing...")
	client.Flush(ctx)

	elapsed := time.Since(start)
	fmt.Printf("✅ Successfully produced %d messages in %v\n", numMessages, elapsed)
	fmt.Printf("   Rate: %.2f messages/sec\n", float64(numMessages)/elapsed.Seconds())
}
