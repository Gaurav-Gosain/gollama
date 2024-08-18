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

	if cfg.Version {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			VERSION = info.Main.Version
		}
		fmt.Println("Gollama version:", helpStyle.
			Render(VERSION),
		)
		return
	}

	if cfg.Install || cfg.Manage || cfg.Monitor {
		cfg.ollamanager()
		return
	}

	// check if the user has provided a model name and prompt
	if cfg.ModelName != "" || cfg.Prompt != "" {
		if cfg.ModelName == "" {
			utils.PrintError(fmt.Errorf("model name is required for generation"), true)
			return
		}

		if cfg.Prompt == "" {
			utils.PrintError(fmt.Errorf("prompt is required for generation"), true)
			return
		}

		cfg.generate()
		return
	}

	tui()
}
