package gollama

import (
	"fmt"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gaurav-gosain/gollama/internal/config"
	"github.com/gaurav-gosain/gollama/internal/model"
)

type Gollama struct {
	Program *tea.Program
	Config  *config.Config
}

func (gollama *Gollama) Init() (err error) {
	gollama.Config = config.NewConfig()
	err = gollama.Config.RunPromptForm()
	if err != nil {
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

func (gollama *Gollama) Generate() {
	gollama.Config.Generate(gollama.Program)
}

func (gollama *Gollama) Run() {
	// wait for the go routine to finish before exiting
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		gollama.Generate()
	}()

	if !gollama.Config.PipedMode && !gollama.Config.Raw {
		resModel, err := gollama.Program.Run()
		if err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
		fmt.Println(model.FinalResponse(resModel.(model.TUI)))
	}

	wg.Wait()
}
