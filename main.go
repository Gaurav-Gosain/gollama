package main

import (
	"fmt"

	"github.com/gaurav-gosain/gollama/internal/gollama"
	"github.com/gaurav-gosain/gollama/internal/utils"
)

func main() {
	gollama := &gollama.Gollama{}
	err := gollama.Init()
	utils.PrintError(err, true)

	response, err := gollama.Run()
	utils.PrintError(err, true)

	if !gollama.Config.Raw {
		fmt.Println(response)
	}
}
