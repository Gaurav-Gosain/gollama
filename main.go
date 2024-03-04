package main

import (
	"fmt"
	"os"

	"github.com/gaurav-gosain/gollama/internal/gollama"
)

func main() {
	gollama := &gollama.Gollama{}
	err := gollama.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing gollama: %s\n", err.Error())
		os.Exit(1)
	}

	gollama.Run()
}
