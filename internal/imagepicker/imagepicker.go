package imagepicker

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type imagepicker struct {
	filepicker   filepicker.Model
	err          error
	SelectedFile string
	Quitting     bool
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func (m imagepicker) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m imagepicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quitting = true
			return m, tea.Quit
		}
	case clearErrorMsg:
		m.err = nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		// generate a full path to the file
		m.SelectedFile = path

		return m, tea.Quit
	}

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not a valid image. Please select a file with a .png or .jpg extension.")
		m.SelectedFile = ""
		return m, tea.Batch(cmd, clearErrorAfter(2*time.Second))
	}

	return m, cmd
}

func (m imagepicker) View() string {
	if m.Quitting || m.SelectedFile != "" {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n  ")
	if m.err != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.err.Error()))
	} else if m.SelectedFile == "" {
		s.WriteString("Pick a file:")
	} else {
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.SelectedFile))
	}
	s.WriteString("\n\n" + m.filepicker.View() + "\n")

	s.WriteString(m.helpView())

	return s.String()
}

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

func (img imagepicker) helpView() string {
	helpViewStr := "\n ←/→: Navigate • Enter: Select File • q: Quit\n"
	return helpStyle(helpViewStr)
}

func Init() (imagepicker, error) {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".png", ".jpg", ".jpeg"}
	fp.AutoHeight = false
	fp.Height = 10
	dir, err := os.Getwd()
	if err != nil {
		return imagepicker{}, err
	}
	fp.CurrentDirectory = dir

	m := imagepicker{
		filepicker: fp,
	}
	res, err := tea.NewProgram(&m).Run()

	return res.(imagepicker), err
}
