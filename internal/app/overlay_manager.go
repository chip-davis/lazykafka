package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	kafkaadmin "mojosoftware.dev/lazykafka/internal/kafka_admin"
	"mojosoftware.dev/lazykafka/internal/ui"
)

type OverlayType int

const (
	OverlayNone OverlayType = iota
	OverlayCreateTopic
	OverlayDeleteTopic
	OverlayProduceMessage
	OverlayDownloadTopic
)

type OverlayManager struct {
	active             OverlayType
	createTopicForm    ui.CreateTopicForm
	deleteTopicForm    ui.DeleteTopicForm
	produceMessageForm ui.ProduceMessageForm
	downloadTopicForm  ui.DownloadTopicForm
	selectedTopic      string
}

func NewOverlayManager() OverlayManager {
	return OverlayManager{
		active:          OverlayNone,
		createTopicForm: ui.NewCreateTopicForm(),
		deleteTopicForm: ui.NewDeleteTopicForm(""),
	}
}

func (om *OverlayManager) IsActive() bool {
	return om.active != OverlayNone
}

func (om *OverlayManager) Close() {
	om.active = OverlayNone
}

func (om *OverlayManager) OpenCreateTopic() {
	om.active = OverlayCreateTopic
	om.createTopicForm = ui.NewCreateTopicForm()
}

func (om *OverlayManager) OpenDeleteTopic(topicName string) {
	om.active = OverlayDeleteTopic
	om.selectedTopic = topicName
	om.deleteTopicForm = ui.NewDeleteTopicForm(topicName)
}

func (om *OverlayManager) OpenProduceMessage(topicName string) {
	om.active = OverlayProduceMessage
	om.selectedTopic = topicName
	om.produceMessageForm = ui.NewProduceMessageForm(topicName)
}

func (om *OverlayManager) OpenDownloadTopic(topicName string) {
	om.active = OverlayDownloadTopic
	om.selectedTopic = topicName
	om.downloadTopicForm = ui.NewDownloadTopicForm(topicName)
}

func (om *OverlayManager) Update(
	msg tea.Msg,
	client *kafkaadmin.Client,
	toastMgr *ToastManager,
	fetchTopicsCmd func(*kafkaadmin.Client) tea.Cmd,
	downloadTopicCmd func(*kafkaadmin.Client, string, string) tea.Cmd,
) (bool, tea.Cmd) {
	if !om.IsActive() {
		return false, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "esc" {
			om.Close()
			return true, nil
		}
	}

	switch om.active {
	case OverlayCreateTopic:
		return om.handleCreateTopic(msg, client, toastMgr, fetchTopicsCmd)
	case OverlayDeleteTopic:
		return om.handleDeleteTopic(msg, client, toastMgr, fetchTopicsCmd)
	case OverlayProduceMessage:
		return om.handleProduceMessage(msg, client, toastMgr)
	case OverlayDownloadTopic:
		return om.handleDownloadTopic(msg, client, toastMgr, downloadTopicCmd)
	}
	return false, nil
}

func (om *OverlayManager) handleCreateTopic(
	msg tea.Msg,
	client *kafkaadmin.Client,
	toastMgr *ToastManager,
	fetchTopicsCmd func(*kafkaadmin.Client) tea.Cmd,
) (bool, tea.Cmd) {
	if topic, ok := msg.(ui.TopicSubmittedMsg); ok {
		om.Close()
		ctx := context.Background()
		_, err := client.CreateTopic(ctx, topic.TopicName)
		if err != nil {
			log.Errorf("Failed to create topic: %v", err)
			return true, nil
		}
		return true, fetchTopicsCmd(client)
	}

	updatedForm, cmd := om.createTopicForm.Update(msg)
	om.createTopicForm = updatedForm.(ui.CreateTopicForm)
	return true, cmd
}

func (om *OverlayManager) handleDeleteTopic(
	msg tea.Msg,
	client *kafkaadmin.Client,
	toastMgr *ToastManager,
	fetchTopicsCmd func(*kafkaadmin.Client) tea.Cmd,
) (bool, tea.Cmd) {
	if deletedMsg, ok := msg.(ui.TopicDeleteMsg); ok {
		om.Close()
		if deletedMsg.Confirmed {
			ctx := context.Background()
			_, err := client.DeleteTopic(ctx, om.selectedTopic)
			if err != nil {
				log.Errorf("Failed to delete topic: %v", err)
				return true, nil
			}
			return true, fetchTopicsCmd(client)
		}
		return true, nil
	}
	updatedForm, cmd := om.deleteTopicForm.Update(msg)
	om.deleteTopicForm = updatedForm.(ui.DeleteTopicForm)
	return true, cmd
}

func (om *OverlayManager) handleProduceMessage(
	msg tea.Msg,
	client *kafkaadmin.Client,
	toastMgr *ToastManager,
) (bool, tea.Cmd) {
	if message, ok := msg.(ui.ProduceMsg); ok {
		om.Close()
		ctx := context.Background()
		record, err := client.BuildRecord(om.selectedTopic, message.PartitionNumber, message.KeySerde, message.ValueSerde,
			message.Key, message.Value, message.Headers)
		if err != nil {
			log.Errorf("Error: %v", err)
		}
		client.ProduceMessage(ctx, &record)
		return true, toastMgr.ShowSuccess("Message produced successfully!")
	}

	updatedForm, cmd := om.produceMessageForm.Update(msg)
	om.produceMessageForm = updatedForm.(ui.ProduceMessageForm)
	return true, cmd
}

func (om *OverlayManager) handleDownloadTopic(
	msg tea.Msg,
	client *kafkaadmin.Client,
	toastMgr *ToastManager,
	downloadTopicCmd func(*kafkaadmin.Client, string, string) tea.Cmd,
) (bool, tea.Cmd) {
	if downloadMsg, ok := msg.(ui.DownloadTopicSubmittedMsg); ok {
		if !downloadMsg.ValidPath {
			return true, toastMgr.ShowError("Error! Download path is not valid")
		}

		om.Close()
		return true, tea.Batch(
			toastMgr.ShowInfo("Download started..."),
			downloadTopicCmd(client, downloadMsg.TopicName, downloadMsg.DownloadPath),
		)
	}

	updatedForm, cmd := om.downloadTopicForm.Update(msg)
	om.downloadTopicForm = updatedForm.(ui.DownloadTopicForm)
	return true, cmd
}

func (om *OverlayManager) View(background string) string {
	if !om.IsActive() {
		return background
	}

	var formView string
	switch om.active {
	case OverlayCreateTopic:
		formView = om.createTopicForm.View()
	case OverlayDeleteTopic:
		formView = om.deleteTopicForm.View()
	case OverlayProduceMessage:
		formView = om.produceMessageForm.View()
	case OverlayDownloadTopic:
		formView = om.downloadTopicForm.View()
	default:
		return background
	}
	return overlay.Composite(
		formView,
		background,
		overlay.Center,
		overlay.Center,
		0,
		0,
	)
}
