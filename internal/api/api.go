package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

type ResultMsg struct {
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

func (r ResultMsg) String() string {
	return r.Response
}

type TagModel struct {
	Details struct {
		Format            string `json:"format"`
		Family            string `json:"family"`
		Families          any    `json:"families"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Digest     string `json:"digest"`
	Size       int64  `json:"size"`
}

type TagResponse struct {
	Models []TagModel `json:"models"`
}

type ApiClient struct {
	baseURL string
}

func NewApiClient(baseURL string) *ApiClient {
	return &ApiClient{baseURL: baseURL}
}

func (a *ApiClient) URL(suffix string) string {
	return a.baseURL + suffix
}

func (a *ApiClient) OllamaModelNames() ([]string, error) {
	url := a.URL("/api/tags")

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(res.Body)
	var tags TagResponse
	err = decoder.Decode(&tags)
	if err != nil {
		return nil, err
	}

	if len(tags.Models) == 0 {
		return nil, errors.New("no models available")
	}

	modelNames := make([]string, 0, len(tags.Models))

	for _, model := range tags.Models {
		modelNames = append(modelNames, model.Name)
	}

	return modelNames, nil
}

func (a *ApiClient) Generate(payloadBytes []byte) (*http.Response, error) {
	url := a.URL("/api/generate")

	payload := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}
