package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

type modelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func GetModelList() ([]string, error) {
	provider, err := LoadDefaultProviderConfig()
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	request := req.C().SetTimeout(15 * time.Second).R()
	modelURL := baseURL + "/models"
	switch {
	case provider.Name == "gemini":
		request.SetQueryParam("key", provider.ApiKey)
	case provider.Type == "anthropic:messages":
		if !strings.HasSuffix(baseURL, "/v1") {
			modelURL = baseURL + "/v1/models"
		}
		request.SetHeader("x-api-key", provider.ApiKey).
			SetHeader("anthropic-version", "2023-06-01")
	default:
		if !strings.HasSuffix(baseURL, "/v1") {
			modelURL = baseURL + "/v1/models"
		}
		request.SetHeader("Authorization", "Bearer "+provider.ApiKey)
	}

	resp, err := request.Get(modelURL)
	if err != nil {
		return nil, fmt.Errorf("get model list: %w", err)
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("get model list: HTTP %d", resp.StatusCode)
	}

	var payload modelListResponse
	if err := json.Unmarshal(resp.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("parse model list: %w", err)
	}
	seen := make(map[string]struct{}, len(payload.Data))
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	for _, item := range payload.Models {
		id := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	sort.Strings(models)
	return models, nil
}
