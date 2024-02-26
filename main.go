package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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

type resultMsg struct {
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

func (r resultMsg) String() string {
	return r.Response
}

type Gollama struct {
	renderer *glamour.TermRenderer
	model    string
	prompt   string
	output   string
	title    string
	viewport viewport.Model
	spinner  spinner.Model
	width    int
	height   int
	state    state
}

func InitSpinner() spinner.Model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	randomSpinner := spinners[rand.Intn(len(spinners))]
	s.Spinner = randomSpinner
	return s
}

func newModel(config Config) (*Gollama, error) {
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

	return &Gollama{
		model:    config.ModelName,
		prompt:   config.Prompt,
		spinner:  InitSpinner(),
		title:    title,
		state:    startState,
		viewport: vp,
		renderer: renderer,
	}, nil
}

func (gollama Gollama) Init() tea.Cmd {
	return gollama.spinner.Tick
}

func (gollama Gollama) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If the user presses q or ctrl+c, we'll quit the program
		switch msg.String() {
		case "ctrl+c", "q":
			gollama.state = quitState
			return gollama, tea.Quit
		default:
			if gollama.state == responseState {
				return gollama, nil
			}
			var cmd tea.Cmd
			gollama.viewport, cmd = gollama.viewport.Update(msg)
			return gollama, cmd
		}
	case resultMsg:
		if msg.Done {
			gollama.state = doneState
			// return m, tea.Quit
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
	default:
		return gollama, nil
	}
}

func FinalResponse(gollama Gollama) string {
	var out string

	out, _ = glamour.Render(gollama.output, "dark")
	return gollama.title + out
}

func (gollama Gollama) View() (render string) {
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

func (gollama Gollama) helpView() string {
	return helpStyle("\n  ↑/↓: Navigate • q: Quit\n")
}

type Payload struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type TagModel struct {
	Details struct {
		Format            string      `json:"format"`
		Family            string      `json:"family"`
		Families          interface{} `json:"families"`
		ParameterSize     string      `json:"parameter_size"`
		QuantizationLevel string      `json:"quantization_level"`
	} `json:"details"`
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Digest     string `json:"digest"`
	Size       int64  `json:"size"`
}

type TagResponse struct {
	Models []TagModel `json:"models"`
}

func NotEmpty(s string) error {
	if s == "" {
		return errors.New("prompt can't be empty")
	}

	// min length should be 10
	if len(s) < 10 {
		return errors.New("prompt should be at least 10 characters long")
	}

	return nil
}

type Config struct {
	Prompt    string
	ModelName string
	PipedMode bool
	Raw       bool
}

func main() {
	var config Config

	flag.BoolVar(&config.PipedMode, "piped", false, "Enable piped mode")
	flag.BoolVar(&config.Raw, "raw", false, "Enable raw output")
	flag.StringVar(&config.Prompt, "prompt", "", "Prompt to use for generation")
	flag.StringVar(&config.ModelName, "model", "", "Model to use for generation")

	flag.Parse()

	var p *tea.Program

	if !config.PipedMode {

		fields := []huh.Field{}

		if config.Prompt == "" {
			fields = append(fields, huh.NewText().
				Title("Prompt").
				// Prompt("Enter a prompt: ").
				Validate(NotEmpty).
				Placeholder("prompt go brr...").
				Value(&config.Prompt),
			)
		}
		if config.ModelName == "" {
			url := "http://localhost:11434/api/tags"

			res, err := http.Get(url)
			if err != nil {
				fmt.Println("Error getting tags:", err)
			}

			decoder := json.NewDecoder(res.Body)
			var tags TagResponse
			err = decoder.Decode(&tags)
			if err != nil {
				fmt.Println("Error decoding response:", err)
				return
			}

			if len(tags.Models) == 0 {
				fmt.Println("No models available")
				return
			}

			modelNames := make([]string, 0, len(tags.Models))

			for _, model := range tags.Models {
				modelNames = append(modelNames, model.Name)
			}
			config.ModelName = modelNames[0]
			fields = append(fields, huh.NewSelect[string]().
				Key("model").
				Options(huh.NewOptions(modelNames...)...).
				Title("Pick an Ollama Model").
				Description("Choose the model to use for generation.").
				Value(&config.ModelName),
			)
		}

		if len(fields) > 0 {

			form := huh.NewForm(
				huh.NewGroup(
					fields...,
				),
			)

			err := form.Run()
			if err != nil {
				fmt.Println("Error running form:", err)
				return
			}
		}

		gollama, err := newModel(config)
		if err != nil {
			fmt.Println("Error creating model:", err)
			return
		}

		p = tea.NewProgram(
			gollama,
		)
	} else {
		if config.Prompt == "" {
			fmt.Println("Prompt can't be empty")
			return
		}
		if config.ModelName == "" {
			fmt.Println("Model can't be empty")
			return
		}
	}

	// wait for the go routine to finish before exiting
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		url := "http://localhost:11434/api/generate"

		newPayload := Payload{
			Model:  config.ModelName,
			Prompt: config.Prompt,
		}

		payloadBytes, err := json.Marshal(newPayload)
		if err != nil {
			fmt.Println("Error marshalling payload:", err)
			return
		}

		payload := bytes.NewReader(payloadBytes)

		req, err := http.NewRequest("POST", url, payload)
		if err != nil {
			fmt.Println("Error creating request:", err)
			return
		}
		req.Header.Add("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("Error making request:", err)
			return
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			fmt.Println("Error: Unexpected status code:", res.StatusCode)
			return
		}

		decoder := json.NewDecoder(res.Body)

		for {
			var resp resultMsg
			err := decoder.Decode(&resp)
			if err != nil {
				fmt.Println("Error decoding response:", err)
				return
			}

			if !config.PipedMode && !config.Raw {
				p.Send(resp)
			} else {
				fmt.Print(resp.Response)
			}

			if resp.Done {
				if config.PipedMode || config.Raw {
					fmt.Println()
				}
				break
			}
		}
	}()

	if !config.PipedMode && !config.Raw {
		resModel, err := p.Run()
		if err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
		fmt.Println(FinalResponse(resModel.(Gollama)))
	}

	wg.Wait()
}
