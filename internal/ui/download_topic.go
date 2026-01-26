package ui

import (
	"io/fs"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DownloadTopicForm struct {
	topicName    string
	downloadPath string
	textInput    textinput.Model
	width        int
	height       int
}

type DownloadTopicSubmittedMsg struct {
	TopicName    string
	DownloadPath string
	ValidPath    bool
}

func NewDownloadTopicForm(topicName string) DownloadTopicForm {
	downloadPathInput := textinput.New()
	downloadPathInput.Placeholder = "Enter file path"
	downloadPathInput.Focus()
	downloadPathInput.Width = 50
	return DownloadTopicForm{
		topicName: topicName,
		textInput: downloadPathInput,
	}
}

func (f DownloadTopicForm) Init() tea.Cmd { return textinput.Blink }
func (f DownloadTopicForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if f.textInput.Value() == "" {
				if fs.ValidPath(f.textInput.Value()) {
					return f, func() tea.Msg {
						return DownloadTopicSubmittedMsg{TopicName: f.topicName, DownloadPath: f.textInput.Value(), ValidPath: true}
					}
				}
			}
		case "esc":
			return f, nil
		}
	}
	f.textInput, cmd = f.textInput.Update(msg)
	return f, cmd
}
func (f DownloadTopicForm) View() string {
	title := FormTitleStyle.Render("Download Topic")
	input := f.textInput.View()
	help := FormHelpStyle.Render("enter: download • esc: cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"File path: ",
		input,
		help,
	)

	return FormBoxStyle.Render(content)
}
