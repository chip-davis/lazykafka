package app

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/twmb/franz-go/pkg/kgo"
	kafkaadmin "mojosoftware.dev/lazykafka/internal/kafka_admin"
)

type TopicItem struct {
	Name string
}

func (t TopicItem) Title() string       { return t.Name }
func (t TopicItem) Description() string { return "" }
func (t TopicItem) FilterValue() string { return t.Name }

type DownloadCompleteMsg struct {
	Success bool
	Err     error
}

type KafkaMessageReceivedMsg struct {
	Records []*kgo.Record
}

type TopicsLoadedMsg struct {
	Items []list.Item
}

func StartConsumerCmd(client *kafkaadmin.Client, ctx context.Context, topicName string, recordChan chan *kgo.Record) tea.Cmd {
	go func() {
		err := client.ConsumeMessages(ctx, topicName, recordChan)
		if err != nil && err != context.Canceled {
			log.Errorf("Consumer error: %v", err)
		}
		close(recordChan)
	}()

	return WaitForMessageCmd(ctx, recordChan)
}

func WaitForMessageCmd(ctx context.Context, recordChan chan *kgo.Record) tea.Cmd {
	return func() tea.Msg {
		batch := make([]*kgo.Record, 0, 1000)

		select {
		case record, ok := <-recordChan:
			if !ok {
				return nil
			}
			batch = append(batch, record)
		case <-ctx.Done():
			return nil
		}

		for len(batch) < 1000 {
			select {
			case record, ok := <-recordChan:
				if !ok {
					return KafkaMessageReceivedMsg{Records: batch}
				}
				batch = append(batch, record)
			default:
				return KafkaMessageReceivedMsg{Records: batch}
			}
		}

		return KafkaMessageReceivedMsg{Records: batch}
	}
}

func FetchTopicsCmd(client *kafkaadmin.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		topicDetails, err := client.ListTopics(ctx)
		if err != nil {
			return nil
		}
		topics := make([]list.Item, 0, len(topicDetails))
		for topicName := range topicDetails {
			topics = append(topics, TopicItem{Name: topicName})
		}
		return TopicsLoadedMsg{Items: topics}
	}
}

func DownloadTopicCmd(client *kafkaadmin.Client, topicName, filePath string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := client.DownloadTopic(ctx, topicName, filePath)
		if err != nil {
			return DownloadCompleteMsg{Success: false, Err: err}
		}
		return DownloadCompleteMsg{Success: true, Err: nil}
	}
}
