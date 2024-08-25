package chatpicker

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gaurav-gosain/gollama/internal/client"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type ExitReason string

// ExitReason is an enum for the exit reasons of the chat picker
const (
	ExitReasonError      ExitReason = "error"
	ExitReasonCancel     ExitReason = "cancel"
	ExitReasonSelect     ExitReason = "select"
	ExitReasonNewChat    ExitReason = "new_chat"
	ExitReasonDeleteChat ExitReason = "delete_chat"
)

// basic bubbletea list model for the chat picker
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
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "esc":
				if m.list.FilterState() != list.FilterApplied {
					m.exitReason = ExitReasonCancel
					return m, tea.Quit
				}
			case "ctrl+c", "q":
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
			case "d":
				i, ok := m.list.SelectedItem().(client.Chat)
				if ok {
					m.exitReason = ExitReasonDeleteChat
					m.selectedChat = i
					return m, tea.Quit
				}
			}
		} else {
			switch msg.String() {
			case "ctrl+c":
				m.exitReason = ExitReasonCancel
				return m, tea.Quit
			case "ctrl+n":
				m.exitReason = ExitReasonNewChat
				return m, tea.Quit
			}
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

// Helper function to create a new chat picker with the provided items
func NewChatPicker(items []list.Item) (client.Chat, ExitReason, error) {
	m := model{list: list.New(
		items,
		list.NewDefaultDelegate(),
		0,
		0,
	)}
	m.list.Title = "Pick a chat"

	additionalKeys := []key.Binding{
		key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("New Chat", "ctrl+n"),
		),
		key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("Delete Chat", "d"),
		),
	}

	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return additionalKeys
	}

	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return additionalKeys
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	var finalModel tea.Model
	var err error
	if finalModel, err = p.Run(); err != nil {
		return client.Chat{}, ExitReasonError, fmt.Errorf("could not run chat picker: %w", err)
	}

	m = finalModel.(model)

	return m.selectedChat, m.exitReason, nil
}
