package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

type CodexStreamEvent struct {
	Type     string
	Delta    string
	Item     map[string]any
	Response map[string]any
}

func RequOuthStream(ctx context.Context, accesstoken string, accountID string, provider string, data string, onEvent func(CodexStreamEvent) error) (error, string) {
	if provider != "oauth:codex" {
		return errors.New("unsupported oauth stream provider"), ""
	}

	reader, writer := io.Pipe()
	client := req.C()
	request := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "text/event-stream").
		SetHeader("Authorization", "Bearer "+accesstoken).
		SetOutput(writer)
	if accountID != "" {
		request.SetHeader("ChatGPT-Account-Id", accountID)
	}

	responseCh := make(chan *req.Response, 1)
	errorCh := make(chan error, 1)
	go func() {
		resp, err := request.SetBody(data).Post("https://chatgpt.com/backend-api/codex/responses")
		_ = writer.CloseWithError(err)
		responseCh <- resp
		errorCh <- err
	}()

	completed, parseErr := parseCodexStream(reader, onEvent)
	resp := <-responseCh
	requestErr := <-errorCh
	if requestErr != nil {
		return requestErr, ""
	}
	if !resp.IsSuccessState() {
		return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}, ""
	}
	if parseErr != nil {
		return parseErr, ""
	}
	if completed == nil {
		return errors.New("codex stream completed without response.completed"), ""
	}
	responseJSON, err := json.Marshal(completed)
	if err != nil {
		return fmt.Errorf("marshal codex completed response: %w", err), ""
	}
	return nil, string(responseJSON)
}

func parseCodexStream(reader io.Reader, onEvent func(CodexStreamEvent) error) (map[string]any, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var completed map[string]any
	outputItems := make([]any, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			return nil, fmt.Errorf("parse codex stream event: %w", err)
		}
		eventType, _ := payload["type"].(string)
		delta, _ := payload["delta"].(string)
		var item map[string]any
		if value, ok := payload["item"].(map[string]any); ok {
			item = value
		}
		var response map[string]any
		if value, ok := payload["response"].(map[string]any); ok {
			response = value
		}
		if eventType == "response.output_item.done" && item != nil {
			outputItems = append(outputItems, item)
		}
		if eventType == "response.completed" {
			completed = response
			if completed != nil && len(outputItems) > 0 {
				completed["output"] = outputItems
			}
		}
		if onEvent != nil {
			if err := onEvent(CodexStreamEvent{Type: eventType, Delta: delta, Item: item, Response: response}); err != nil {
				return nil, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read codex stream: %w", err)
	}
	return completed, nil
}

func RequOuth(accesstoken string, accountID string, provider string, data string) (error, string) {
	switch provider {
	case "oauth:codex":
		client := req.C()
		request := client.SetTimeout(30*time.Second).R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Authorization", "Bearer "+accesstoken)
		if accountID != "" {
			request.SetHeader("ChatGPT-Account-Id", accountID)
		}
		resp, err := request.
			SetBody(data).
			Post("https://chatgpt.com/backend-api/codex/responses")
		if err != nil {
			return err, ""
		}
		if !resp.IsSuccessState() {
			return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}, ""
		}
		return nil, resp.String()
	case "oauth:anthropic":
	}
	return nil, ""
}
