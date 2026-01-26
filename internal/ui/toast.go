package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ToastType int
type Position int

const (
	Success ToastType = iota
	Error
	Info
)

const (
	TopLeft Position = iota
	TopRight
	BottomLeft
	BottomRight
	Center
)

type Toast struct {
	message   string
	toastType ToastType
	position  Position
	visible   bool
	time      int
}

type ToastTimeoutMsg struct{}

func NewToast(message string, toastType ToastType, position Position, timeout int) Toast {
	return Toast{
		message:   message,
		toastType: toastType,
		position:  position,
		visible:   true,
		time:      timeout * int(time.Second),
	}
}

func (t Toast) Init() tea.Cmd {
	return tea.Tick(time.Duration(t.position), func(time.Time) tea.Msg {
		return ToastTimeoutMsg{}
	})
}

func (t Toast) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ToastTimeoutMsg:
		t.visible = false
		return t, nil
	}
	return t, nil
}

func (t Toast) View() string {
	if !t.visible {
		return ""
	}

	var bgColor lipgloss.Color
	var icon string

	switch t.toastType {
	case Success:
		bgColor = lipgloss.Color("42")
		icon = "✓"
	case Error:
		bgColor = lipgloss.Color("196") // Red
		icon = "✗"
	case Info:
		bgColor = lipgloss.Color("86") // Cyan
		icon = "ℹ"
	}

	style := lipgloss.NewStyle().
		Background(bgColor).
		Foreground(lipgloss.Color("0")).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bgColor)
	content := icon + " " + t.message
	return style.Render(content)
}

func (t Toast) IsVisible() bool {
	return t.visible
}
