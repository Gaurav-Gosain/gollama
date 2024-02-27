package main

import (
	"fmt"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gaurav-gosain/gollama/internal/config"
	"github.com/gaurav-gosain/gollama/internal/model"
)

func main() {
	gollama := config.NewConfig()

	var p *tea.Program

	if !gollama.PipedMode {

		err := gollama.RunPromptForm()

		if err != nil {
			fmt.Println("Error running form:", err.Error())
			return
		}

		gollamaTUI, err := model.NewModel(gollama)
		if err != nil {
			fmt.Println("Error creating model:", err)
			return
		}

		p = tea.NewProgram(
			gollamaTUI,
		)
	} else {
		if gollama.Prompt == "" {
			fmt.Println("Prompt can't be empty")
			return
		}
		if gollama.ModelName == "" {
			fmt.Println("Model can't be empty")
			return
		}
	}

	// wait for the go routine to finish before exiting
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		gollama.Generate(p)
	}()

	if !gollama.PipedMode && !gollama.Raw {
		resModel, err := p.Run()
		if err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
		fmt.Println(model.FinalResponse(resModel.(model.Gollama)))
	}

	wg.Wait()
}
