package model

import (
	"fmt"
	"math/rand"
	"regexp"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/gaurav-gosain/gollama/internal/api"
	"github.com/gaurav-gosain/gollama/internal/config"
	"golang.design/x/clipboard"
)

type state int

const (
	startState state = iota
	responseState
	doneState
	quitState
)

var spinners = []spinner.Spinner{
	spinner.Line,
	spinner.Dot,
	spinner.MiniDot,
	spinner.Jump,
	spinner.Pulse,
	spinner.Points,
	spinner.Globe,
	spinner.Moon,
	spinner.Monkey,
	spinner.Meter,
	spinner.Ellipsis,
	spinner.Hamburger,
}

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

type TUI struct {
	renderer          *glamour.TermRenderer
	model             string
	prompt            string
	output            string
	title             string
	viewport          viewport.Model
	views             []string
	spinner           spinner.Model
	currentViewIndex  int
	state             state
	copiedToClipboard bool
}

func (gollama *TUI) FindCodeBlocks() {
	var codeBlocks []string

	codeBlocks = append(codeBlocks, gollama.output)

	// regex to find code blocks
	re := regexp.MustCompile("```[\\s\\S]*?```")
	matches := re.FindAllString(gollama.output, -1)

	codeBlocks = append(codeBlocks, matches...)

	gollama.views = codeBlocks
}

func InitSpinner() spinner.Model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	randomSpinner := spinners[rand.Intn(len(spinners))]
	s.Spinner = randomSpinner
	return s
}

func NewModel(config *config.Config) (*TUI, error) {
	out := fmt.Sprintf("# Prompt\n`%s`\n# Response\n", config.Prompt)

	title, _ := glamour.Render(out, "dark")

	const width = 78

	vp := viewport.New(width, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}

	return &TUI{
		model:    config.ModelName,
		prompt:   config.Prompt,
		spinner:  InitSpinner(),
		title:    title,
		state:    startState,
		viewport: vp,
		renderer: renderer,
	}, nil
}

func (gollama TUI) Init() tea.Cmd {
	return gollama.spinner.Tick
}

func (gollama *TUI) CopyToClipboard() {
	// cross-platform clipboard copy
	err := clipboard.Init()
	if err != nil {
		panic(err)
	}

	clipboard.Write(clipboard.FmtText, []byte(gollama.views[gollama.currentViewIndex]))

	gollama.copiedToClipboard = true
}

func (gollama *TUI) NavigateView(direction int) {
	gollama.currentViewIndex += direction
	if gollama.currentViewIndex < 0 {
		gollama.currentViewIndex = len(gollama.views) - 1
	}
	if gollama.currentViewIndex >= len(gollama.views) {
		gollama.currentViewIndex = 0
	}

	glamOutput, _ := gollama.renderer.Render(gollama.views[gollama.currentViewIndex])
	gollama.viewport.SetContent(glamOutput)
	gollama.viewport.GotoTop()
}

func (gollama TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case api.ResultMsg:
		if msg.Done {
			gollama.state = doneState

			gollama.FindCodeBlocks()

			return gollama, nil
		}

		if msg.Response == "" {
			return gollama, nil
		}

		if gollama.state == startState {
			gollama.state = responseState
		}

		gollama.output = gollama.output + msg.Response

		currLineCount := gollama.viewport.TotalLineCount()

		glamOutput, _ := gollama.renderer.Render(gollama.output)
		gollama.viewport.SetContent(glamOutput)

		if currLineCount < gollama.viewport.TotalLineCount() {
			gollama.viewport.GotoBottom()
		}
		return gollama, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		gollama.spinner, cmd = gollama.spinner.Update(msg)
		return gollama, cmd
	case tea.KeyMsg:
		if gollama.state == responseState {
			return gollama, nil
		}
		gollama.copiedToClipboard = false
		// If the user presses q or ctrl+c, we'll quit the program
		switch msg.String() {
		case "ctrl+c", "q":
			gollama.state = quitState
			return gollama, tea.Quit
		case "c":
			if len(gollama.views) > 0 {
				gollama.CopyToClipboard()
			}
		case "left", "h":
			if len(gollama.views) > 0 {
				gollama.NavigateView(-1)
			}
			var cmd tea.Cmd
			gollama.viewport, cmd = gollama.viewport.Update(msg)
			return gollama, cmd
		case "right", "l":
			if len(gollama.views) > 0 {
				gollama.NavigateView(1)
			}
			var cmd tea.Cmd
			gollama.viewport, cmd = gollama.viewport.Update(msg)
			return gollama, cmd
		default:
			if gollama.state == responseState {
				return gollama, nil
			}
			var cmd tea.Cmd
			gollama.viewport, cmd = gollama.viewport.Update(msg)
			return gollama, cmd
		}
	default:
		return gollama, nil
	}

	return gollama, nil
}

func (gollama TUI) View() (render string) {
	switch gollama.state {
	case startState:
		out, _ := glamour.Render(fmt.Sprintf("Waiting for response from `%s` model...", gollama.model), "dark")
		return fmt.Sprintf("  %s %s", gollama.spinner.View(), out[1:])
	case responseState:
		return gollama.title + gollama.viewport.View()
	case doneState:
		return gollama.title + gollama.viewport.View() + gollama.helpView()
	case quitState:
		fallthrough
	default:
		return render
	}
}

func (gollama TUI) helpView() string {
	helpViewStr := "\n  ↑/↓: Navigate • q: Quit • c: Copy %s\n"

	if len(gollama.views) > 1 {
		helpViewStr = fmt.Sprintf(
			helpViewStr,
			fmt.Sprintf(
				"• ←/→: Navigate code blocks (%d / %d)\n",
				gollama.currentViewIndex+1,
				len(gollama.views),
			),
		)
	} else {
		helpViewStr = fmt.Sprintf(helpViewStr, "")
	}

	if gollama.copiedToClipboard {
		helpViewStr = helpViewStr + "  Copied to clipboard!\n"
	}
	return helpStyle(helpViewStr)
}

func FinalResponse(gollama TUI) string {
	var out string

	out, _ = glamour.Render(gollama.output, "dark")
	return gollama.title + out
}
