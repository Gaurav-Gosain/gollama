package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/gaurav-gosain/gollama/internal/api"
)

var STRING_DEFAULT = ""
var BOOL_DEFAULT = false

// Config represents the configuration options for the program
type Config struct {
	Prompt    string
	ModelName string
	PipedMode bool
	Raw       bool
	BaseURL   string
	ApiClient *api.ApiClient
}

func NewConfig() *Config {
	config := Config{}
	config.ParseCLIArgs()
	config.ApiClient = api.NewApiClient(config.BaseURL)

	return &config
}

func (c *Config) ParseCLIArgs() {
	// Parse command line flags
	flag.BoolVar(&c.PipedMode, "piped", BOOL_DEFAULT, "Enable piped mode")
	flag.BoolVar(&c.Raw, "raw", BOOL_DEFAULT, "Enable raw output")
	flag.StringVar(&c.Prompt, "prompt", STRING_DEFAULT, "Prompt to use for generation")
	flag.StringVar(&c.ModelName, "model", STRING_DEFAULT, "Model to use for generation")
	flag.StringVar(&c.BaseURL, "base-url", "http://localhost:11434", "Base URL for the API server")

	flag.Parse()
}

func validate(s string) error {
	if s == "" {
		return errors.New("prompt can't be empty")
	}

	// min length should be 10
	if len(s) < 10 {
		return errors.New("prompt should be at least 10 characters long")
	}

	return nil
}

func (c *Config) GetFormFields() (fields []huh.Field, err error) {
	if c.ModelName == STRING_DEFAULT {
		modelNames, err := c.ApiClient.OllamaModelNames()

		if err != nil {
			return nil, err
		}
		c.ModelName = modelNames[0]
		fields = append(fields, huh.NewSelect[string]().
			Key("model").
			Options(huh.NewOptions(modelNames...)...).
			Title("Pick an Ollama Model").
			Description("Choose the model to use for generation.").
			Value(&c.ModelName),
		)
	}
	if c.Prompt == STRING_DEFAULT {
		fields = append(fields, huh.NewText().
			Title("Prompt").
			// Prompt("Enter a prompt: ").
			Validate(validate).
			Placeholder("prompt go brr...").
			Value(&c.Prompt),
		)
	}

	return fields, nil
}

type Payload struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

func (c *Config) RunPromptForm() (err error) {

	fields, err := c.GetFormFields()

	if err != nil {
		return err
	}

	if len(fields) > 0 {

		form := huh.NewForm(
			huh.NewGroup(
				fields...,
			),
		)

		err = form.Run()
		if err != nil {
			return
		}
	}

	return
}

func (c *Config) Generate(p *tea.Program) {

	newPayload := Payload{
		Model:  c.ModelName,
		Prompt: c.Prompt,
	}

	payloadBytes, err := json.Marshal(newPayload)
	if err != nil {
		fmt.Println("Error marshalling payload:", err)
		return
	}

	generateResponse, err := c.ApiClient.Generate(payloadBytes)

	if err != nil {
		fmt.Println("Error generating response:", err)
		return
	}

	defer generateResponse.Body.Close()

	if generateResponse.StatusCode != http.StatusOK {
		fmt.Println("Error generating response:", generateResponse.Status)
		return
	}

	decoder := json.NewDecoder(generateResponse.Body)

	for {
		var resp api.ResultMsg
		err := decoder.Decode(&resp)
		if err != nil {
			fmt.Println("Error decoding response:", err)
			return
		}

		if !c.PipedMode && !c.Raw {
			p.Send(resp)
		} else {
			fmt.Print(resp.Response)
		}

		if resp.Done {
			if c.PipedMode || c.Raw {
				fmt.Println()
			}
			break
		}
	}
}
