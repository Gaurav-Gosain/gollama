package chatpicker

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gaurav-gosain/gollama/internal/client"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type ExitReason string

const (
	ExitReasonError   ExitReason = "error"
	ExitReasonCancel  ExitReason = "cancel"
	ExitReasonSelect  ExitReason = "select"
	ExitReasonNewChat ExitReason = "new_chat"
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list         list.Model
	exitReason   ExitReason
	selectedChat client.Chat
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.exitReason = ExitReasonCancel
			return m, tea.Quit
		case "ctrl+n":
			m.exitReason = ExitReasonNewChat
			return m, tea.Quit
		case "enter":
			i, ok := m.list.SelectedItem().(client.Chat)
			if ok {
				m.selectedChat = i
				m.exitReason = ExitReasonSelect
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func NewChatPicker(items []list.Item) (client.Chat, ExitReason, error) {
	m := model{list: list.New(
		items,
		list.NewDefaultDelegate(),
		0,
		0,
	)}
	m.list.Title = "Pick a chat"

	p := tea.NewProgram(m, tea.WithAltScreen())

	var finalModel tea.Model
	var err error
	if finalModel, err = p.Run(); err != nil {
		return client.Chat{}, ExitReasonError, fmt.Errorf("could not run chat picker: %w", err)
	}

	m = finalModel.(model)

	return m.selectedChat, m.exitReason, nil
}
