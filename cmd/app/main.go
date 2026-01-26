package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/twmb/franz-go/pkg/kgo"
	kafkaadmin "mojosoftware.dev/lazykafka/internal/kafka_admin"
	"mojosoftware.dev/lazykafka/internal/ui"
)

type overlayType int

const (
	overlayNone overlayType = iota
	overlayCreateTopic
	overlayDeleteTopic
	overlayProduceMessage
	overlayDownloadTopic
)

type viewState int

const (
	viewTopicsList viewState = iota
	viewTopicDetail
)

type model struct {
	list             list.Model
	bootstrapServers string
	client           *kafkaadmin.Client
	width            int
	height           int

	// View state
	currentView     viewState
	topicViewModels map[string]*ui.TopicViewModel
	activeConsumers map[string]context.CancelFunc
	consumerCtx     context.Context
	consumerCancel  context.CancelFunc
	messageChan     chan *kgo.Record

	// Overlay
	overlayMgr    OverlayManager
	selectedTopic string

	toastMgr ui.ToastManager
}

type OverlayManager struct {
	active             overlayType
	createTopicForm    ui.CreateTopicForm
	deleteTopicForm    ui.DeleteTopicForm
	produceMessageForm ui.ProduceMessageForm
	downloadTopicForm  ui.DownloadTopicForm
	selectedTopic      string
}

func newOverlayManager() OverlayManager {
	return OverlayManager{
		active:          overlayNone,
		createTopicForm: ui.NewCreateTopicForm(),
		deleteTopicForm: ui.NewDeleteTopicForm(""),
	}
}

func (om *OverlayManager) IsActive() bool {
	return om.active != overlayNone
}

func (om *OverlayManager) Close() {
	om.active = overlayNone
}

func (om *OverlayManager) OpenCreateTopic() {
	om.active = overlayCreateTopic
	om.createTopicForm = ui.NewCreateTopicForm()
}

func (om *OverlayManager) OpenDeleteTopic(topicName string) {
	om.active = overlayDeleteTopic
	om.selectedTopic = topicName
	om.deleteTopicForm = ui.NewDeleteTopicForm(topicName)
}

func (om *OverlayManager) OpenProduceMessage(topicName string) {
	om.active = overlayProduceMessage
	om.selectedTopic = topicName
	om.produceMessageForm = ui.NewProduceMessageForm(topicName)
}

func (om *OverlayManager) OpenDownloadTopic(topicName string) {
	om.active = overlayDownloadTopic
	om.selectedTopic = topicName
	om.downloadTopicForm = ui.NewDownloadTopicForm(topicName)
}

func (om *OverlayManager) Update(msg tea.Msg, client *kafkaadmin.Client, toastMgr *ui.ToastManager) (bool, tea.Cmd) {
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
	case overlayCreateTopic:
		return om.handleCreateTopic(msg, client, toastMgr)
	case overlayDeleteTopic:
		return om.handleDeleteTopic(msg, client, toastMgr)
	case overlayProduceMessage:
		return om.handleProduceMessage(msg, client, toastMgr)
	case overlayDownloadTopic:
		return om.handleDownloadTopic(msg, client, toastMgr)
	}
	return false, nil
}

func (om *OverlayManager) handleCreateTopic(msg tea.Msg, client *kafkaadmin.Client, toastMgr *ui.ToastManager) (bool, tea.Cmd) {
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

func (om *OverlayManager) handleDeleteTopic(msg tea.Msg, client *kafkaadmin.Client, toastMgr *ui.ToastManager) (bool, tea.Cmd) {
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

func (om *OverlayManager) handleProduceMessage(msg tea.Msg, client *kafkaadmin.Client, toastMgr *ui.ToastManager) (bool, tea.Cmd) {
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

func (om *OverlayManager) handleDownloadTopic(msg tea.Msg, client *kafkaadmin.Client, toastMgr *ui.ToastManager) (bool, tea.Cmd) {
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
	case overlayCreateTopic:
		formView = om.createTopicForm.View()
	case overlayDeleteTopic:
		formView = om.deleteTopicForm.View()
	case overlayProduceMessage:
		formView = om.produceMessageForm.View()
	case overlayDownloadTopic:
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

type topicsLoadedMsg struct {
	items []list.Item
}

type kafkaMessageReceivedMsg struct {
	records []*kgo.Record
}

type downloadCompleteMsg struct {
	success bool
	err     error
}

type topicItem struct {
	name string
}

func (t topicItem) Title() string       { return t.name }
func (t topicItem) Description() string { return "" }
func (t topicItem) FilterValue() string { return t.name }

func initialModel(bootstrapServers string, kafkaAdmin *kafkaadmin.Client) model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Topics"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return model{
		bootstrapServers: bootstrapServers,
		client:           kafkaAdmin,
		list:             l,
		currentView:      viewTopicsList,
		topicViewModels:  make(map[string]*ui.TopicViewModel),
		activeConsumers:  make(map[string]context.CancelFunc),
		toastMgr:         ui.NewToastManager(),
		overlayMgr:       newOverlayManager(),
	}
}

func startConsumerCmd(client *kafkaadmin.Client, ctx context.Context, topicName string, recordChan chan *kgo.Record) tea.Cmd {
	go func() {
		err := client.ConsumeMessages(ctx, topicName, recordChan)
		if err != nil && err != context.Canceled {
			log.Errorf("Consumer error: %v", err)
		}
		close(recordChan)
	}()

	return waitForMessageCmd(ctx, recordChan)
}

func waitForMessageCmd(ctx context.Context, recordChan chan *kgo.Record) tea.Cmd {
	return func() tea.Msg {
		batch := make([]*kgo.Record, 0, 100)

		select {
		case record, ok := <-recordChan:
			if !ok {
				return nil
			}
			batch = append(batch, record)
		case <-ctx.Done():
			return nil
		}

		for len(batch) < 100 {
			select {
			case record, ok := <-recordChan:
				if !ok {
					return kafkaMessageReceivedMsg{records: batch}
				}
				batch = append(batch, record)
			default:
				return kafkaMessageReceivedMsg{records: batch}
			}
		}

		return kafkaMessageReceivedMsg{records: batch}
	}
}

func fetchTopicsCmd(client *kafkaadmin.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		topicDetails, err := client.ListTopics(ctx)
		if err != nil {
			return nil
		}
		topics := make([]list.Item, 0, len(topicDetails))
		for topicName := range topicDetails {
			topics = append(topics, topicItem{name: topicName})
		}
		return topicsLoadedMsg{items: topics}
	}
}

func downloadTopicCmd(client *kafkaadmin.Client, topicName, filePath string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := client.DownloadTopic(ctx, topicName, filePath)
		if err != nil {
			return downloadCompleteMsg{success: false, err: err}
		}
		return downloadCompleteMsg{success: true, err: nil}
	}
}

func (m model) Init() tea.Cmd {
	return fetchTopicsCmd(m.client)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	if m.toastMgr.HandleMessage(msg) {
		return m, nil
	}

	if m.currentView == viewTopicDetail {

		if kafkaMsg, ok := msg.(kafkaMessageReceivedMsg); ok {
			if len(kafkaMsg.records) > 0 {
				if vm, exists := m.topicViewModels[m.selectedTopic]; exists {
					vm.AddMessages(kafkaMsg.records)
				}
				return m, waitForMessageCmd(m.consumerCtx, m.messageChan)
			}
		}

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "esc" {
				m.currentView = viewTopicsList
				return m, nil
			}
		}

		if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = windowMsg.Width
			m.height = windowMsg.Height
		}

		if vm, exists := m.topicViewModels[m.selectedTopic]; exists {
			updatedModel, cmd := vm.Update(msg)
			m.topicViewModels[m.selectedTopic] = updatedModel.(*ui.TopicViewModel)
			return m, cmd
		}
		return m, nil
	}

	if handled, cmd := m.overlayMgr.Update(msg, m.client, &m.toastMgr); handled {
		return m, cmd
	}

	switch msg := msg.(type) {

	case topicsLoadedMsg:
		m.list.SetItems(msg.items)
		return m, nil

	case downloadCompleteMsg:
		if msg.success {
			return m, m.toastMgr.ShowSuccess("Download completed successfully!")
		} else {
			return m, m.toastMgr.ShowError(fmt.Sprintf("Download failed: %v", msg.err))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		h := msg.Height - 10 // Leave room for header, help, padding
		m.list.SetWidth(msg.Width - 8)
		m.list.SetHeight(h)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "c": // create topic
			m.overlayMgr.OpenCreateTopic()
			return m, nil

		case "p": // produce message
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(topicItem)
				m.selectedTopic = topic.name
				m.overlayMgr.OpenProduceMessage(topic.name)
				return m, nil
			}

		case "d": // download topic
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(topicItem)
				m.selectedTopic = topic.name
				m.overlayMgr.OpenDownloadTopic(topic.name)
				return m, nil
			}

		case "x", "X": // delete topic
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(topicItem)
				m.selectedTopic = topic.name
				m.overlayMgr.OpenDeleteTopic(topic.name)
				return m, nil
			}

		case "enter":
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(topicItem)
				m.selectedTopic = topic.name
				m.currentView = viewTopicDetail

				if _, exists := m.topicViewModels[topic.name]; !exists {
					m.topicViewModels[topic.name] = ui.NewTopicViewModel(topic.name, m.width, m.height)
				}

				if _, exists := m.activeConsumers[topic.name]; !exists {
					m.consumerCtx, m.consumerCancel = context.WithCancel(context.Background())
					m.messageChan = make(chan *kgo.Record, 100)

					m.activeConsumers[topic.name] = m.consumerCancel

					return m, startConsumerCmd(m.client, m.consumerCtx, topic.name, m.messageChan)
				}

				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.currentView == viewTopicDetail {
		if vm, exists := m.topicViewModels[m.selectedTopic]; exists {
			return m.toastMgr.Wrap(vm.View())
		}
		return m.toastMgr.Wrap("Error: Topic view model not found")
	}

	header := ui.HeaderStyle.Width(m.width - 4).Render(
		fmt.Sprintf("%s %s",
			ui.TitleStyle.Render("lazykafka"),
			fmt.Sprintf("→ %s", m.bootstrapServers),
		),
	)

	listPanel := ui.PanelStyle.
		Width(m.width - 8).
		Height(m.height - 8).
		Render(m.list.View())

	help := ui.HelpStyle.Render("↑/↓ j/k: navigate • /: filter • c: create topic • x: delete topic • p: produce message • d: download topic • q: quit")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listPanel,
		help,
	)

	background := ui.AppStyle.Render(content)

	return m.toastMgr.Wrap(m.overlayMgr.View(background))
}

func main() {
	if len(os.Args) < 2 {
		log.Error("Must provide bootstrapservers")
		os.Exit(1)
	}

	bootstrapServers := os.Args[1]

	adminClient, err := kafkaadmin.NewClient(bootstrapServers)
	if err != nil {
		log.Errorf("Failed to create admin client: %v", err)
		os.Exit(1)
	}
	defer adminClient.Close()

	p := tea.NewProgram(
		initialModel(bootstrapServers, adminClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
