package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/gaurav-gosain/gollama/internal/api"
	"github.com/gaurav-gosain/gollama/internal/imagepicker"
)

const (
	STRING_DEFAULT = ""
	BOOL_DEFAULT   = false
)

// Config represents the configuration options for the program
type Config struct {
	ApiClient  *api.ApiClient
	Prompt     string
	ModelName  string
	BaseURL    string
	PipedText  string
	ImagePath  string
	Images     []string
	MultiModal bool
	PipedMode  bool
	Install    bool
	Raw        bool
}

func NewConfig() *Config {
	config := Config{}
	config.ParseCLIArgs()
	config.GetPipedInput()
	config.ApiClient = api.NewApiClient(config.BaseURL)

	return &config
}

func (c *Config) GetPipedInput() {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting standard input information: %v\n", err)
		os.Exit(1)
	}

	// Check if there is data available to read
	if (fileInfo.Mode()&os.ModeNamedPipe != 0) || (fileInfo.Mode()&os.ModeCharDevice == 0) {
		c.PipedMode = true
		c.Raw = true

		pipedData, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading standard input: %v\n", err)
			os.Exit(1)
		}

		if c.Prompt == "" {
			c.Prompt = string(pipedData)
		} else {
			c.Prompt = fmt.Sprintf("Context: %s\n\nQuestion: %s", pipedData, c.Prompt)
		}
	}
}

func (c *Config) ParseCLIArgs() {
	// Parse command line flags
	flag.BoolVar(&c.Raw, "raw", BOOL_DEFAULT, "Enable raw output")
	flag.BoolVar(
		&c.MultiModal,
		"attach-image",
		BOOL_DEFAULT,
		"Allow attaching an image (automatically set to true if an image path is provided)",
	)
	flag.StringVar(&c.Prompt, "prompt", STRING_DEFAULT, "Prompt to use for generation")
	flag.StringVar(&c.BaseURL, "base-url", "http://localhost:11434", "Base URL for the API server")
	flag.StringVar(&c.ImagePath, "image", STRING_DEFAULT, "Path to the image file to attach (png/jpg/jpeg)")
	flag.StringVar(&c.ModelName, "model", STRING_DEFAULT, "Model to use for generation")
	flag.BoolVar(&c.Install, "install", BOOL_DEFAULT, "Install an Ollama Model")

	flag.Parse()

	// if path starts with `~` then expand it to the home directory
	if strings.HasPrefix(c.ImagePath, "~/") {
		dirname, _ := os.UserHomeDir()
		c.ImagePath = filepath.Join(dirname, c.ImagePath[2:])
	}

	if c.ImagePath != STRING_DEFAULT {
		c.MultiModal = true
	}
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

func (c *Config) GetFormFields() (fields []huh.Field, pickFile bool, err error) {
	if c.PipedMode {
		return fields, false, nil
	}

	if c.ModelName == STRING_DEFAULT {
		modelNames, err := c.ApiClient.OllamaModelNames()
		if err != nil {
			return nil, false, err
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

	if c.MultiModal && c.ImagePath == STRING_DEFAULT {
		pickFile = true
	}

	return fields, pickFile, nil
}

type Payload struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
}

func (c *Config) RunPromptForm() (err error) {
	if c.PipedMode && c.ModelName == "" {
		return errors.New("model name can't be empty when running in piped mode")
	}

	fields, pickFile, err := c.GetFormFields()
	if err != nil {
		return err
	}

	if pickFile {
		satisfied := false

		for !satisfied {
			img, err := imagepicker.Init()
			if err != nil {
				return err
			}

			if img.Quitting {
				return errors.New("user quit the file picker")
			}

			c.ImagePath = img.SelectedFile

			huh.NewConfirm().Value(&satisfied).Title(fmt.Sprintf("Selected file: %s", c.ImagePath)).Run()
		}
	}

	if len(fields) > 0 {

		if c.ImagePath != STRING_DEFAULT {
			fields = append([]huh.Field{
				huh.NewNote().
					Title(fmt.Sprintf("Selected file: %s", c.ImagePath)),
			}, fields...)
		}

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

	if c.MultiModal && c.ImagePath != STRING_DEFAULT {
		bytes, err := os.ReadFile(c.ImagePath)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		base64Encoding := base64.StdEncoding.EncodeToString(bytes)

		c.Images = []string{base64Encoding}
	}

	return
}

func (c *Config) Generate(p *tea.Program) {
	newPayload := Payload{
		Model:  c.ModelName,
		Prompt: c.Prompt,
		Images: c.Images,
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

		if c.Raw {
			fmt.Print(resp.Response)
		} else {
			p.Send(resp)
		}

		if resp.Done {
			if c.Raw {
				fmt.Println()
			}
			break
		}
	}
}
