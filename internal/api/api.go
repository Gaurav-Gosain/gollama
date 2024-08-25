package api

import (
	"github.com/ollama/ollama/api"
)

type OllamaAPI struct {
	Client *api.Client
}

// creates a new OllamaAPI instance, uses the official Ollama go client, loads
// the environment variables
func NewOllamaAPI() (OllamaAPI, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return OllamaAPI{}, err
	}

	return OllamaAPI{
		Client: client,
	}, nil
}
