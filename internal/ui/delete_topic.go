package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DeleteTopicForm struct {
	topicName string
}

type TopicDeleteMsg struct {
	Confirmed bool
}

func NewDeleteTopicForm(topicName string) DeleteTopicForm {
	return DeleteTopicForm{
		topicName: topicName,
	}
}

func (f DeleteTopicForm) Init() tea.Cmd { return nil }

func (f DeleteTopicForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y":
			return f, func() tea.Msg {
				return TopicDeleteMsg{Confirmed: true}
			}
		case "n", "N", "esc":
			return f, func() tea.Msg {
				return TopicDeleteMsg{Confirmed: false}
			}
		}
	}
	return f, nil
}

func (f DeleteTopicForm) View() string {
	title := FormTitleStyle.Render("Delete Topic")
	warning := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("⚠ Warning")
	message := fmt.Sprintf("Are you sure you want to delete topic:\n%s", f.topicName)
	help := FormHelpStyle.Render("y: confirm • n/esc: cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		warning,
		message,
		"",
		help,
	)
	return FormBoxStyle.Render(content)
}
