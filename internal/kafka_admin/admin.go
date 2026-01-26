package kafkaadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kversion"
)

type Client struct {
	kgoClient  *kgo.Client
	admClient  *kadm.Client
	bootstraps string
}

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

func NewClient(bootstrapServers string) (*Client, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrapServers),
		kgo.MaxVersions(kversion.V2_4_0()),
	)
	if err != nil {
		return nil, err
	}

	adminClient := kadm.NewClient(client)

	return &Client{
		kgoClient:  client,
		admClient:  adminClient,
		bootstraps: bootstrapServers,
	}, nil
}

func (c *Client) ListTopics(ctx context.Context) (kadm.TopicDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	topicDetails, err := c.admClient.ListTopics(ctx)
	if err != nil {
		log.Errorf("failed to list topics: %v", err)
		return nil, err
	}
	return topicDetails, nil
}

func (c *Client) CreateTopic(ctx context.Context, topicName string) (kadm.CreateTopicResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	topicDetails, err := c.ListTopics(ctx)
	if err != nil {
		log.Errorf("failed to create topics: %v", err)
		return kadm.CreateTopicResponse{}, err
	}

	if topicDetails.Has(topicName) {
		log.Infof("topic %v already exists\n", topicName)
		return kadm.CreateTopicResponse{}, err
	}

	log.Debugf("creating %v topic\n", topicName)
	createTopicResponse, err := c.admClient.CreateTopic(ctx, -1, -1, nil, topicName)
	if err != nil {
		log.Errorf("failed to create topic: %v", err)
		return kadm.CreateTopicResponse{}, err
	}
	log.Debugf("Successfully created topic %v\n", createTopicResponse.Topic)
	return createTopicResponse, nil
}

func (c *Client) DeleteTopic(ctx context.Context, topicName string) (kadm.DeleteTopicResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	deleteTopicResponse, err := c.admClient.DeleteTopic(ctx, topicName)
	if err != nil {
		return kadm.DeleteTopicResponse{}, err
	}
	return deleteTopicResponse, nil
}

func (c *Client) BuildRecord(topicName string, partitionNumber string, keySerde string, valueSerde string, key string, value string, headers string) (kgo.Record, error) {
	if value == "" {
		log.Errorf("Value must not be null")
	}

	partitionNumberInt, err := strconv.Atoi(partitionNumber)
	if err != nil {
		return kgo.Record{}, err
	}

	r := kgo.Record{
		Partition: int32(partitionNumberInt),
		Key:       []byte(key),
		Value:     []byte(value),
		Topic:     topicName,
	}
	return r, nil

}

func (c *Client) ProduceMessage(ctx context.Context, record *kgo.Record) {
	ctx = context.Background()
	c.kgoClient.Produce(ctx, record, func(r *kgo.Record, err error) {
		if err != nil {
			log.Errorf("Error producing message: %v", err)
		}
	})
	c.kgoClient.Flush(ctx)
}

func (c *Client) ConsumeMessages(ctx context.Context, topicName string, recordChan chan<- *kgo.Record) error {
	opts := []kgo.Opt{
		kgo.ConsumeTopics(topicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.SeedBrokers(c.bootstraps),
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("unable to create client: %w", err)
	}
	defer cl.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return nil
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, err := range errs {
				log.Errorf("fetch error: %v", err)
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			select {
			case recordChan <- record:
			case <-ctx.Done():
				return
			}
		})
	}
}

func (c *Client) DownloadTopic(ctx context.Context, topicName string, filePath string) error {
	opts := []kgo.Opt{
		kgo.ConsumeTopics(topicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.SeedBrokers(c.bootstraps),
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("unable to create client: %w", err)
	}
	defer cl.Close()

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("unable to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	file.WriteString("[\n")
	firstRecord := true

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			file.WriteString("\n]")
			return nil
		default:
		}

		fetches := cl.PollFetches(timeoutCtx)
		if fetches.IsClientClosed() {
			file.WriteString("\n]")
			return nil
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, err := range errs {
				log.Errorf("fetch error: %v", err)
			}
			continue
		}

		recordCount := 0
		fetches.EachRecord(func(record *kgo.Record) {
			recordCount++

			if !firstRecord {
				file.WriteString(",\n")
			}
			firstRecord = false

			recordData := map[string]interface{}{
				"partition": record.Partition,
				"offset":    record.Offset,
				"timestamp": record.Timestamp.Format(time.RFC3339),
				"key":       string(record.Key),
				"value":     string(record.Value),
			}

			encoder.Encode(recordData)
		})

		if recordCount == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (c *Client) Close() {
	c.admClient.Close()
	c.kgoClient.Close()
}
