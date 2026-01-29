package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/twmb/franz-go/pkg/kgo"
	"mojosoftware.dev/lazykafka/internal/app"
	kafkaadmin "mojosoftware.dev/lazykafka/internal/kafka_admin"
	"mojosoftware.dev/lazykafka/internal/ui"
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
	overlayMgr    app.OverlayManager
	selectedTopic string

	toastMgr app.ToastManager
}

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
		toastMgr:         app.NewToastManager(),
		overlayMgr:       app.NewOverlayManager(),
	}
}

func (m model) Init() tea.Cmd {
	return app.FetchTopicsCmd(m.client)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	if m.toastMgr.HandleMessage(msg) {
		return m, nil
	}

	if m.currentView == viewTopicDetail {

		if kafkaMsg, ok := msg.(app.KafkaMessageReceivedMsg); ok {
			if len(kafkaMsg.Records) > 0 {
				if vm, exists := m.topicViewModels[m.selectedTopic]; exists {
					vm.AddMessages(kafkaMsg.Records)
				}
				return m, app.WaitForMessageCmd(m.consumerCtx, m.messageChan)
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

	if handled, cmd := m.overlayMgr.Update(msg, m.client, &m.toastMgr, app.FetchTopicsCmd, app.DownloadTopicCmd); handled {
		return m, cmd
	}

	switch msg := msg.(type) {

	case app.TopicsLoadedMsg:
		m.list.SetItems(msg.Items)
		return m, nil

	case app.DownloadCompleteMsg:
		if msg.Success {
			return m, m.toastMgr.ShowSuccess("Download completed successfully!")
		} else {
			return m, m.toastMgr.ShowError(fmt.Sprintf("Download failed: %v", msg.Err))
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
				topic := selectedItem.(app.TopicItem)
				m.selectedTopic = topic.Name
				m.overlayMgr.OpenProduceMessage(topic.Name)
				return m, nil
			}

		case "d": // download topic
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(app.TopicItem)
				m.selectedTopic = topic.Name
				m.overlayMgr.OpenDownloadTopic(topic.Name)
				return m, nil
			}

		case "x", "X": // delete topic
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(app.TopicItem)
				m.selectedTopic = topic.Name
				m.overlayMgr.OpenDeleteTopic(topic.Name)
				return m, nil
			}

		case "enter":
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				topic := selectedItem.(app.TopicItem)
				m.selectedTopic = topic.Name
				m.currentView = viewTopicDetail

				if _, exists := m.topicViewModels[topic.Name]; !exists {
					m.topicViewModels[topic.Name] = ui.NewTopicViewModel(topic.Name, m.width, m.height)
				}

				if _, exists := m.activeConsumers[topic.Name]; !exists {
					m.consumerCtx, m.consumerCancel = context.WithCancel(context.Background())
					m.messageChan = make(chan *kgo.Record, 10000)

					m.activeConsumers[topic.Name] = m.consumerCancel

					return m, app.StartConsumerCmd(m.client, m.consumerCtx, topic.Name, m.messageChan)
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
