package ui

import "github.com/charmbracelet/lipgloss"

var (
	PrimaryColor = lipgloss.Color("170") // Purple
	AccentColor  = lipgloss.Color("86")  // Cyan
	SubtleColor  = lipgloss.Color("241") // Gray
	BorderColor  = lipgloss.Color("62")  // Purple-ish

	AppStyle = lipgloss.NewStyle().
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderBottom(true).
			BorderForeground(BorderColor).
			MarginBottom(1).
			Padding(0, 1)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			MarginTop(1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	FormBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2).
			Width(50)

	FormTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1)

	FormHelpStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			MarginTop(1)
)
