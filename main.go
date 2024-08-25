package main

import (
	"fmt"
	"runtime/debug"

	"github.com/gaurav-gosain/gollama/internal/utils"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func main() {
	cfg := &gollamaConfig{}

	cfg.ParseCLIArgs()

	// if the version flag is set, print the version and exit
	if cfg.Version {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			VERSION = info.Main.Version // set the version variable, if available
		}
		fmt.Println("Gollama version:", helpStyle.
			Render(VERSION),
		)
		return
	}

	// any ollamanager related flags branch to the ollamanager process and exit
	if cfg.Install || cfg.Manage || cfg.Monitor {
		cfg.ollamanager()
		return
	}

	// checks if the user has provided a model name and prompt
	// if either the model name or prompt is empty, print an error and exit
	if cfg.ModelName != "" || cfg.Prompt != "" {
		if cfg.ModelName == "" {
			utils.PrintError(fmt.Errorf("model name is required for generation"), true)
			return
		}

		if cfg.Prompt == "" {
			utils.PrintError(fmt.Errorf("prompt is required for generation"), true)
			return
		}

		// calls the generate function to print the generated text to stdout
		cfg.generate()
		return
	}

	// the default case is to start the TUI
	tui()
}
