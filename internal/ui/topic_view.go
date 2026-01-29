package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	messageCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(0, 1).
				MarginBottom(1)

	messageHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	messageLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Width(12)

	messageValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Padding(2, 0)
)

type TopicViewModel struct {
	topicName string
	messages  []*kgo.Record
	viewport  viewport.Model
	width     int
	height    int

	currentPage int
	pageSize    int

	searchMode  bool
	searchInput textinput.Model
	searchTerm  string

	searchProgress bool
	filteredCache  []*kgo.Record
}

type SearchResultMsg struct {
	results []*kgo.Record
	query   string
}

func SearchMessagesCmd(messages []*kgo.Record, query string) tea.Cmd {
	return func() tea.Msg {
		if query == "" {
			return SearchResultMsg{results: messages, query: query}
		}

		pattern := []rune(query)

		type scoredMatch struct {
			record *kgo.Record
			score  int
		}
		matches := make([]scoredMatch, 0)

		for _, msg := range messages {
			valueStr := string(msg.Value)
			input := util.RunesToChars([]rune(valueStr))

			result, _ := algo.FuzzyMatchV2(false, false, true, &input, pattern, false, nil)
			if result.Start >= 0 {
				matches = append(matches, scoredMatch{record: msg, score: result.Score})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})
		results := make([]*kgo.Record, len(matches))
		for i, match := range matches {
			results[i] = match.record
		}
		return SearchResultMsg{results: results, query: query}
	}
}

func (t *TopicViewModel) AddMessage(record *kgo.Record) {
	t.messages = append(t.messages, record)
}

func (t *TopicViewModel) AddMessages(records []*kgo.Record) {
	t.messages = append(t.messages, records...)
}

func (t *TopicViewModel) filteredMessages() []*kgo.Record {
	if t.searchTerm == "" {
		return t.messages
	}
	return t.filteredCache
}

func (t *TopicViewModel) totalPages() int {
	filtered := t.filteredMessages()
	if len(filtered) == 0 {
		return 1
	}
	return (len(filtered) + t.pageSize - 1) / t.pageSize
}

func (t *TopicViewModel) nextPage() {
	if t.currentPage < t.totalPages()-1 {
		t.currentPage++
		t.viewport.GotoTop()
	}
}

func (t *TopicViewModel) prevPage() {
	if t.currentPage > 0 {
		t.currentPage--
		t.viewport.GotoTop()
	}
}

func NewTopicViewModel(topicName string, width, height int) *TopicViewModel {
	vp := viewport.New(width, height-6)
	vp.SetContent("")

	searchInput := textinput.New()
	searchInput.Placeholder = "Search in keys or values..."
	searchInput.Width = 50

	return &TopicViewModel{
		topicName:   topicName,
		messages:    make([]*kgo.Record, 0),
		viewport:    vp,
		width:       width,
		height:      height,
		currentPage: 0,
		pageSize:    500,
		searchMode:  false,
		searchInput: searchInput,
		searchTerm:  "",
	}
}

func (t *TopicViewModel) Init() tea.Cmd {
	return nil
}

func (t *TopicViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.viewport.Width = msg.Width
		t.viewport.Height = msg.Height - 6
		t.width = msg.Width
		t.height = msg.Height

	case SearchResultMsg:
		if msg.query == t.searchTerm {
			t.searchProgress = false
			t.filteredCache = msg.results
		}
		return t, nil
	case tea.KeyMsg:
		if t.searchMode {
			switch msg.String() {
			case "enter":
				t.searchTerm = t.searchInput.Value()
				t.searchMode = false
				t.searchProgress = true
				t.currentPage = 0
				return t, SearchMessagesCmd(t.messages, t.searchTerm)
			case "esc":
				t.searchMode = false
				t.searchInput.SetValue("")
				return t, nil
			default:
				t.searchInput, cmd = t.searchInput.Update(msg)
				return t, cmd
			}
		}

		switch msg.String() {
		case "/":
			t.searchMode = true
			t.searchInput.Focus()
			t.searchInput.SetValue("")
			return t, textinput.Blink
		case "n":
			t.nextPage()
			return t, nil
		case "p":
			t.prevPage()
			return t, nil
		case "c":
			if t.searchTerm != "" {
				t.searchTerm = ""
				t.searchInput.SetValue("")
				t.currentPage = 0
				return t, nil
			}
		case "g":
			t.viewport.GotoTop()
			return t, nil
		case "G":
			t.viewport.GotoBottom()
			return t, nil
		default:
			t.viewport, cmd = t.viewport.Update(msg)
		}
	}

	return t, cmd
}

func (t *TopicViewModel) renderMessages() string {
	if len(t.messages) == 0 {
		return emptyStateStyle.Render("⏳ Waiting for messages...")
	}

	filtered := t.filteredMessages()

	if len(filtered) == 0 {
		return emptyStateStyle.Render(fmt.Sprintf("No messages found matching '%s'", t.searchTerm))
	}

	var content strings.Builder
	cardWidth := t.viewport.Width - 4

	start := t.currentPage * t.pageSize
	end := start + t.pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	if start >= len(filtered) {
		t.currentPage = t.totalPages() - 1
		start = t.currentPage * t.pageSize
		end = start + t.pageSize
		if end > len(filtered) {
			end = len(filtered)
		}
	}

	for i := start; i < end; i++ {
		record := filtered[i]
		originalIdx := -1
		for idx, msg := range t.messages {
			if msg == record {
				originalIdx = idx
				break
			}
		}
		msgNumber := originalIdx + 1
		if originalIdx == -1 {
			msgNumber = i + 1
		}
		header := messageHeaderStyle.Render(fmt.Sprintf("Message #%d", msgNumber))

		timestamp := time.Unix(0, record.Timestamp.UnixNano()).Format("2006-01-02 15:04:05")

		meta := fmt.Sprintf("%s %s\n",
			messageLabelStyle.Render("Timestamp:"),
			messageValueStyle.Render(timestamp))
		meta += fmt.Sprintf("%s %s\n",
			messageLabelStyle.Render("Partition:"),
			messageValueStyle.Render(fmt.Sprintf("%d", record.Partition)))
		meta += fmt.Sprintf("%s %s\n",
			messageLabelStyle.Render("Offset:"),
			messageValueStyle.Render(fmt.Sprintf("%d", record.Offset)))

		keyStr := string(record.Key)
		if keyStr == "" {
			keyStr = "(null)"
		}
		meta += fmt.Sprintf("%s %s\n",
			messageLabelStyle.Render("Key:"),
			messageValueStyle.Render(keyStr))

		valueStr := string(record.Value)
		if len(valueStr) > 200 {
			valueStr = valueStr[:200] + "..."
		}
		meta += fmt.Sprintf("%s %s",
			messageLabelStyle.Render("Value:"),
			messageValueStyle.Render(valueStr))

		cardContent := lipgloss.JoinVertical(lipgloss.Left, header, meta)
		card := messageCardStyle.Width(cardWidth).Render(cardContent)
		content.WriteString(card)
		content.WriteString("\n")
	}

	return content.String()
}

func (t *TopicViewModel) View() string {
	t.viewport.SetContent(t.renderMessages())

	filtered := t.filteredMessages()
	headerText := fmt.Sprintf("📨 %s • %d total", t.topicName, len(t.messages))

	if t.searchTerm != "" {
		headerText += fmt.Sprintf(" • %d filtered", len(filtered))
	}

	totalPages := t.totalPages()
	if totalPages > 1 {
		headerText += fmt.Sprintf(" • Page %d/%d", t.currentPage+1, totalPages)
	}

	header := HeaderStyle.Width(t.width).Render(headerText)

	var searchBar string
	if t.searchMode {
		searchBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Render("Search: ") + t.searchInput.View()
		searchBar = lipgloss.NewStyle().
			Padding(0, 1).
			Render(searchBar)
	} else if t.searchTerm != "" {
		searchBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1).
			Render(fmt.Sprintf("🔍 Searching: '%s' (press 'c' to clear)", t.searchTerm))
	} else if t.searchProgress {
		searchBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1).
			Render("🔍 Searching...")
	}

	viewportView := t.viewport.View()

	scrollPercent := fmt.Sprintf("%3.f%%", t.viewport.ScrollPercent()*100)
	help := HelpStyle.Render(fmt.Sprintf(
		"↑/↓ j/k: scroll • g/G: top/bottom • n/p: next/prev page • /: search • c: clear search • %s • esc: back",
		scrollPercent))

	parts := []string{header}
	if searchBar != "" {
		parts = append(parts, searchBar)
	}
	parts = append(parts, viewportView, help)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		parts...,
	)
}
