package kafkaadmin

import (
	"context"
	"fmt"
	"os"
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

func (c *Client) Close() {
	c.admClient.Close()
	c.kgoClient.Close()
}
