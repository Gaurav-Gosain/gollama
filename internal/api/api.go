package api

import (
	"github.com/ollama/ollama/api"
)

type OllamaAPI struct {
	Client *api.Client
}

func NewOllamaAPI() (OllamaAPI, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return OllamaAPI{}, err
	}

	return OllamaAPI{
		Client: client,
	}, nil
}
