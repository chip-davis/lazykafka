package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var fields = []string{
	"PartitionNumber",
	"KeySerde",
	"ValueSerde",
	"Key",
	"Value",
	"Headers",
}

var labels = []string{
	"Partition Number",
	"Key Serde",
	"Value Serde",
	"Key",
	"Value",
	"Headers",
}

type ProduceMessageValues struct {
	PartitionNumber string
	KeySerde        string
	ValueSerde      string
	Key             string
	Value           string
	Headers         string
}

type formInput struct {
	field string
	input textinput.Model
}

type ProduceMessageForm struct {
	width     int
	height    int
	topicName string
	focused   int
	inputs    []formInput
}

type ProduceMsg struct {
	TopicName       string
	PartitionNumber string
	KeySerde        string
	ValueSerde      string
	Key             string
	Value           string
	Headers         string
}

func NewProduceMessageForm(topicName string) ProduceMessageForm {

	inputs := make([]formInput, len(fields))
	for i := range fields {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		ti.Width = 60
		if labels[i] != "Value" {
			ti.CharLimit = 100
		}
		inputs[i] = formInput{
			field: fields[i],
			input: ti,
		}
	}

	inputs[0].input.Focus()

	return ProduceMessageForm{
		topicName: topicName,
		inputs:    inputs,
		focused:   0,
		width:     80,
		height:    30,
	}
}

func (f ProduceMessageForm) Init() tea.Cmd { return textinput.Blink }
func (f ProduceMessageForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			f.inputs[f.focused].input.Blur()
			f.focused = (f.focused + 1) % len(f.inputs)

			cmd = f.inputs[f.focused].input.Focus()
			return f, cmd
		case "shift+tab":
			f.inputs[f.focused].input.Blur()
			f.focused = (f.focused - 1 + len(f.inputs)) % len(f.inputs)
			cmd = f.inputs[f.focused].input.Focus()
			return f, cmd
		case "enter":
			values := ProduceMessageValues{}

			for _, fi := range f.inputs {
				switch fi.field {
				case "PartitionNumber":
					values.PartitionNumber = fi.input.Value()
				case "KeySerde":
					values.KeySerde = fi.input.Value()
				case "ValueSerde":
					values.ValueSerde = fi.input.Value()
				case "Key":
					values.Key = fi.input.Value()
				case "Value":
					values.Value = fi.input.Value()
				case "Headers":
					values.Headers = fi.input.Value()
				}
			}

			return f, func() tea.Msg {
				return ProduceMsg{
					TopicName:       f.topicName,
					PartitionNumber: values.PartitionNumber,
					KeySerde:        values.KeySerde,
					ValueSerde:      values.ValueSerde,
					Key:             values.Key,
					Value:           values.Value,
					Headers:         values.Headers,
				}
			}
		case "esc":
			return f, nil
		}
	}
	ti, cmd := f.inputs[f.focused].input.Update(msg)
	f.inputs[f.focused].input = ti
	return f, cmd
}

func (f ProduceMessageForm) View() string {
	title := FormTitleStyle.Render("Produce new message")

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	var renderedInputs []string
	for i, fi := range f.inputs {
		label := labelStyle.Render(labels[i])
		input := fi.input.View()
		renderedInputs = append(renderedInputs, label)
		renderedInputs = append(renderedInputs, input)
	}

	help := FormHelpStyle.Render(
		"enter: produce message • esc: cancel • tab: switch focus",
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		append(
			[]string{title},
			append(renderedInputs, help)...,
		)...,
	)

	return lipgloss.Place(
		f.width,
		f.height,
		lipgloss.Center,
		lipgloss.Center,
		FormBoxStyle.Render(content),
	)
}
