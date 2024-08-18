package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gaurav-gosain/gollama/internal/api"
	"github.com/gaurav-gosain/gollama/internal/utils"
	oapi "github.com/ollama/ollama/api"
)

func (cfg *gollamaConfig) generate() {
	ollamaAPI, err := api.NewOllamaAPI()
	if err != nil {
		utils.PrintError(err, true)
	}

	msg := strings.TrimSpace(cfg.Prompt)

	imageData := []oapi.ImageData{}
	for _, img := range cfg.Images {
		expandedPath, err := utils.ExpandPath(img)
		if err != nil {
			continue
		}
		imgData, err := os.ReadFile(expandedPath)
		if err == nil {
			imageData = append(imageData, imgData)
		}
	}

	chatRequest := oapi.GenerateRequest{
		Model:  cfg.ModelName,
		Prompt: msg,
		Images: imageData,
	}

	ctx := context.Background()

	ollamaAPI.Client.Generate(ctx, &chatRequest, func(response oapi.GenerateResponse) error {
		fmt.Print(response.Response)
		return nil
	})
	fmt.Println()
}
