package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/net/html"
)

var (
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1F1F1")).
			Background(lipgloss.Color("#8839ef")).
			Bold(true).
			Padding(0, 1)
	checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓ ")
)

const (
	padding  = 2
	maxWidth = 80
)

// Response represents the response structure from the API
type Response struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

type progressErrMsg struct{ err error }

func finalPause() tea.Cmd {
	return tea.Tick(time.Millisecond*750, func(_ time.Time) tea.Msg {
		return nil
	})
}

type model struct {
	err           error
	status        string
	rawStatus     string
	spinner       spinner.Model
	progress      progress.Model
	isDownloading bool
}

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
	spinner.Hamburger,
}

func InitSpinner() spinner.Model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	randomSpinner := spinners[rand.Intn(len(spinners))]
	s.Spinner = randomSpinner
	return s
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.err = errors.New("user quit mid download :(")
			return m, tea.Quit
		default:
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case progressErrMsg:
		m.err = msg.err
		return m, tea.Quit

	case Response:
		var cmds []tea.Cmd

		if m.rawStatus != msg.Status {
			m.spinner = InitSpinner()
			cmds = append(cmds, m.spinner.Tick)
			if m.status != "" {
				cmds = append(cmds, tea.Println(strings.Repeat(" ", padding), checkMark, m.status))
			}
			switch msg.Status {
			case "pulling manifest":
				m.status = "Pulling manifest..."
			case "verifying sha256 digest":
				m.status = "Verifying sha256 digest..."
			case "writing manifest":
				m.status = "Writing manifest..."
			case "removing any unused layers":
				m.status = "Removing any unused layers..."
			case "success":
				m.status = "Success!"
				cmds = append(cmds, tea.Sequence(finalPause(), tea.Quit))
			default:
				m.status = "Downloading... (" + msg.Status + ") "
			}
		}

		m.isDownloading = msg.Total != 0
		m.rawStatus = msg.Status

		if m.isDownloading {
			progress := float64(msg.Completed) / float64(msg.Total)
			cmds = append(cmds, m.progress.SetPercent(progress))
		}

		return m, tea.Batch(cmds...)

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.err != nil {
		// utils.PrintError(m.err, true)
		return ""
	}

	if m.rawStatus == "success" {
		return ""
	}

	pad := strings.Repeat(" ", padding)

	status := statusStyle.SetString(m.status).String()

	if m.isDownloading {
		return fmt.Sprintf(
			"\n%s%s  %s\n\n%s%s\n\n",
			pad, m.spinner.View(), status,
			pad, m.progress.View(),
		)
	}

	return fmt.Sprintf(
		"\n%s%s  %s\n\n",
		pad, m.spinner.View(), status,
	)
}

type OllamaModel struct {
	Name        string
	Description string
}

func extractModels(htmlString string) []OllamaModel {
	var models []OllamaModel

	// Parse the HTML
	doc, err := html.Parse(strings.NewReader(htmlString))
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return models
	}

	// Define a function to traverse the HTML tree and extract models
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			// Check if the <a> tag has an href attribute starting with "/library/"
			var href, name string
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, "/library/") {
					href = attr.Val
					name = strings.TrimPrefix(href, "/library/")
					break
				}
			}
			if href != "" {
				// Find the <p> tag inside the <a> tag
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "p" {
						// Extract the description text
						description := strings.TrimSpace(c.FirstChild.Data)
						models = append(models, OllamaModel{Name: name, Description: description})
						break
					}
				}
			}
		}
		// Recursively call the function for child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	// Start traversing the HTML tree
	traverse(doc)

	return models
}

func GetAvailableModels() ([]OllamaModel, error) {
	resp, err := http.Get("https://ollama.com/library")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	models := extractModels(string(body))
	return models, nil
}

func GetAvailableTags(modelName string) ([]string, error) {
	resp, err := http.Get("https://ollama.com/library/" + modelName + "/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(body), "\n")
	var items []string
	for _, line := range lines {
		if strings.Contains(line, `href="/library/`+modelName+":") {
			href := strings.Split(line, `href="`)[1]
			item := strings.Split(href, `"`)[0]
			item = strings.TrimPrefix(item, "/library/"+modelName+":")
			items = append(items, item)
		}
	}

	return items, nil
}

func Ollamanager(apiUrl string) (string, string, error) {
	models, err := GetAvailableModels()
	if err != nil {
		fmt.Println("Error getting models")
		return "", "", err
	}

	var modelName string
	confirm := false

	options := []huh.Option[string]{}

	for _, model := range models {
		options = append(options, huh.NewOption(model.Name, model.Name))
	}

	form := huh.NewForm(
		huh.NewGroup(
			// Ask the user for a base burger and toppings.
			huh.NewSelect[string]().
				Title("Choose your model").
				Options(
					options...,
				).
				Value(&modelName), // store the chosen option in the "modelName" variable
		),
	)

	err = form.Run()
	if err != nil {
		return "", "", err
	}

	var tag string
	confirm = false

	modelTags, err := GetAvailableTags(modelName)
	if err != nil {
		fmt.Println("Error getting tags")
		return "", "", err
	}

	options = []huh.Option[string]{}

	for _, modelTag := range modelTags {
		options = append(options, huh.NewOption(modelTag, modelTag))
	}

	form = huh.NewForm(
		huh.NewGroup(
			// Ask the user for a base burger and toppings.
			huh.NewSelect[string]().
				Title("Choose your tag for "+modelName).
				Options(
					options...,
				).
				Value(&tag), // store the chosen option in the "modelName" variable

			huh.NewConfirm().
				Title("Would you like to continue?").
				Value(&confirm),
		),
	)

	err = form.Run()
	if err != nil {
		return "", "", err
	}

	if !confirm {
		return "", "", errors.New("see you")
	}
	downloadingModel := fmt.Sprintf("%s:%s", modelName, tag)

	fmt.Println("Starting to download ", statusStyle.SetString(downloadingModel).String())

	m := model{
		progress: progress.New(progress.WithDefaultGradient()),
		spinner:  InitSpinner(),
	}
	// Start Bubble Tea
	p := tea.NewProgram(m)

	go func(apiUrl string, modelName string, p *tea.Program) {
		// Prepare request body
		requestBody, err := json.Marshal(map[string]string{
			"name": modelName,
		})
		if err != nil {
			fmt.Println("Error marshalling request body:", err)
			return
		}

		// Send POST request to the API
		resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(requestBody))
		if err != nil {
			fmt.Println("Error sending POST request:", err)
			return
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)

		for {
			var response Response
			if err := decoder.Decode(&response); err != nil {
				fmt.Println("Error decoding response:", err)
				p.Quit()
				return
			}

			p.Send(response)

			if response.Status == "success" {
				break
			}
		}
	}(apiUrl, downloadingModel, p)

	res, err := p.Run()
	if err != nil {
		fmt.Println("error running program:", err)
		return "", "", err
	}

	return modelName, tag, res.(model).err
}
