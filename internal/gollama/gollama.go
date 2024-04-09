package gollama

import (
	"fmt"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gaurav-gosain/gollama/internal/config"
	"github.com/gaurav-gosain/gollama/internal/model"
)

type Gollama struct {
	Program *tea.Program
	Config  *config.Config
}

func (gollama *Gollama) PrintError(err error, exitOnErr bool) {
	ErrPadding := lipgloss.NewStyle().Padding(1, 2)
	ErrorHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F1F1F1")).
		Background(lipgloss.Color("#FF5F87")).
		Bold(true).
		Padding(0, 1).
		SetString("ERROR")

	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			ErrPadding.Render(
				fmt.Sprintf(
					"\n%s %s",
					ErrorHeader.String(),
					err.Error(),
				),
			),
		)
		if exitOnErr {
			os.Exit(1)
		}
	}
}

func (gollama *Gollama) Init() (err error) {
	gollama.Config = config.NewConfig()
	err = gollama.Config.RunPromptForm()
	if err != nil {
		return
	}

	if gollama.Config.PipedMode || gollama.Config.Raw {
		return
	}

	gollamaTUI, err := model.NewModel(gollama.Config)
	if err != nil {
		return
	}

	gollama.Program = tea.NewProgram(
		gollamaTUI,
		// TODO: decide if we want to use the altscreen or not
		// tea.WithAltScreen(), // Use the altscreen.
	)

	return
}

func (gollama *Gollama) generate() {
	gollama.Config.Generate(gollama.Program)
}

func (gollama *Gollama) Run() (string, error) {
	// wait for the go routine to finish before exiting
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		gollama.generate()
	}()

	if gollama.Config.PipedMode || gollama.Config.Raw {
		wg.Wait()
		return "", nil
	}

	resModel, err := gollama.Program.Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		return "", err
	}

	wg.Wait()
	return model.FinalResponse(resModel.(model.TUI)), nil
}
