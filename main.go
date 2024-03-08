package main

import (
	"fmt"

	"github.com/gaurav-gosain/gollama/internal/gollama"
)

func main() {
	gollama := &gollama.Gollama{}
	err := gollama.Init()
	gollama.PrintError(err, true)

	response, err := gollama.Run()
	gollama.PrintError(err, true)

	if !gollama.Config.Raw {
		fmt.Println(response)
	}
}
