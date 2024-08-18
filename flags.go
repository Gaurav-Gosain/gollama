package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	flag "github.com/spf13/pflag"
)

var VERSION = "unknown (built from source)"

type gollamaConfig struct {
	Version   bool
	Manage    bool
	Install   bool
	Monitor   bool
	Prompt    string
	ModelName string
	Images    []string
}

var helpStyle = lipgloss.
	NewStyle().
	Padding(0, 1).
	Background(lipgloss.Color("#8839ef")).
	Foreground(lipgloss.Color("#FFFFFF"))

func Highlight(s string, highlight string) string {
	return fmt.Sprintf(s, lipgloss.
		NewStyle().
		Foreground(lipgloss.Color("420")).
		Render(highlight))
}

func (c *gollamaConfig) ParseCLIArgs() {
	// Parse command line flags
	flag.BoolVarP(&c.Version, "version", "v", false, Highlight(
		"Prints the %sersion of Gollama",
		"v",
	))
	flag.BoolVarP(&c.Manage, "manage", "m", false, Highlight(
		"%sanages the installed Ollama models (update/delete installed models)",
		"m",
	))
	flag.BoolVarP(&c.Install, "install", "i", false, Highlight(
		"%snstalls an Ollama model (download and install a model)",
		"i",
	))
	flag.BoolVarP(&c.Monitor, "monitor", "r", false, Highlight(
		"Monitor the status of %sunning Ollama models",
		"r",
	))

	flag.StringVar(&c.ModelName, "model", "", "Model to use for generation")
	flag.StringVar(&c.Prompt, "prompt", "", "Prompt to use for generation")
	flag.StringSliceVar(&c.Images, "images", []string{}, "Paths to the image files to attach (png/jpg/jpeg), comma separated")

	flag.ErrHelp = errors.New("\n" + helpStyle.Render("Gollama's help & usage menu"))
	flag.CommandLine.SortFlags = false

	flag.Parse()

	c.GetPipedInput()
}

func (c *gollamaConfig) GetPipedInput() {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting standard input information: %v\n", err)
		os.Exit(1)
	}

	// Check if there is data available to read
	if (fileInfo.Mode()&os.ModeNamedPipe != 0) || (fileInfo.Mode()&os.ModeCharDevice == 0) {

		pipedData, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading standard input: %v\n", err)
			os.Exit(1)
		}

		if c.Prompt == "" {
			c.Prompt = string(pipedData)
		} else {
			c.Prompt = fmt.Sprintf("Context: %s\n\nQuestion: %s", pipedData, c.Prompt)
		}
	}
}
