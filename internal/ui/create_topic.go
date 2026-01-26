package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CreateTopicForm struct {
	textInput textinput.Model
	width     int
	height    int
}

type TopicSubmittedMsg struct {
	TopicName string
}

func (f CreateTopicForm) Init() tea.Cmd { return textinput.Blink }
func (f CreateTopicForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			return f, nil
		case "enter":
			if f.textInput.Value() != "" {
				return f, func() tea.Msg {
					return TopicSubmittedMsg{TopicName: f.textInput.Value()}
				}
			}
		case "esc":
			return f, nil
		}
	}

	f.textInput, cmd = f.textInput.Update(msg)
	return f, cmd
}
func (f CreateTopicForm) View() string {
	title := FormTitleStyle.Render("Create New Topic")
	input := f.textInput.View()
	help := FormHelpStyle.Render("enter: create • esc: cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"Topic Name:",
		input,
		help,
	)

	return FormBoxStyle.Render(content)
}

func NewCreateTopicForm() CreateTopicForm {
	topicNameInput := textinput.New()
	topicNameInput.Placeholder = "Enter topic name"
	topicNameInput.Focus()
	topicNameInput.CharLimit = 100
	topicNameInput.Width = 50
	return CreateTopicForm{
		textInput: topicNameInput,
	}
}
