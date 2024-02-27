package main

import (
	"fmt"
	"io"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gaurav-gosain/gollama/internal/config"
	"github.com/gaurav-gosain/gollama/internal/model"
)

func main() {
	var p *tea.Program

	gollama := config.NewConfig()

	if gollama.PipedMode {
		if gollama.ModelName == "" {
			fmt.Println("Model can't be empty when running in piped mode")
			return
		}

		pipedData, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading standard input: %v\n", err)
			os.Exit(1)
		}

		if gollama.Prompt == "" {
			gollama.Prompt = string(pipedData)
		} else {
			gollama.Prompt = fmt.Sprintf("Context: %s\n\nQuestion: %s", pipedData, gollama.Prompt)
		}
	} else {
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
			// TODO: decide if we want to use the altscreen or not
			tea.WithAltScreen(), // Use the altscreen.
		)
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
