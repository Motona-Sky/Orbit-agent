package llm

import "orbit/internal/tools"

type openAIResponsesTextFormat struct {
	Type string `json:"type"`
}

type openAIResponsesText struct {
	Format openAIResponsesTextFormat `json:"format"`
}

type openAIResponsesReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type openAIResponsesTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type OpenaiResponse struct {
	Model             string                    `json:"model"`
	Instructions      string                    `json:"instructions,omitempty"`
	Input             []any                     `json:"input"`
	Stream            bool                      `json:"stream"`
	Store             *bool                     `json:"store,omitempty"`
	ParallelToolCalls *bool                     `json:"parallel_tool_calls,omitempty"`
	Include           []string                  `json:"include,omitempty"`
	MaxOutputTokens   *int                      `json:"max_output_tokens,omitempty"`
	Temperature       *float64                  `json:"temperature,omitempty"`
	TopP              *float64                  `json:"top_p,omitempty"`
	Text              openAIResponsesText       `json:"text"`
	Reasoning         *openAIResponsesReasoning `json:"reasoning,omitempty"`
	Tools             []openAIResponsesTool     `json:"tools,omitempty"`
	ToolChoice        string                    `json:"tool_choice,omitempty"`
}

func buildOpenAIResponsesInput(userInput string, memory []MemoryMessage, systemRole string) []any {
	input := make([]any, 0, len(memory)+1)
	for _, message := range memory {
		if len(message.ResponseItems) > 0 {
			input = append(input, message.ResponseItems...)
		}
		role := message.Role
		if role == "system" && systemRole != "" {
			role = systemRole
		}
		switch {
		case len(message.ToolCalls) > 0:
			if message.Content != "" {
				input = append(input, map[string]any{
					"role":    role,
					"content": message.Content,
				})
			}
			for _, value := range message.ToolCalls {
				toolCall, ok := value.(map[string]any)
				if !ok {
					continue
				}
				function, ok := toolCall["function"].(map[string]any)
				if !ok {
					continue
				}
				input = append(input, map[string]any{
					"type":      "function_call",
					"call_id":   toolCall["id"],
					"name":      function["name"],
					"arguments": function["arguments"],
				})
			}
		case message.Role == "tool":
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": message.ToolCallID,
				"output":  message.Content,
			})
		default:
			input = append(input, map[string]any{
				"role":    role,
				"content": message.Content,
			})
		}
	}
	if userInput != "" {
		input = append(input, map[string]any{
			"role":    "user",
			"content": userInput,
		})
	}
	return input
}

func buildOpenAIResponsesTools(registeredTools []tools.ToolReg) []openAIResponsesTool {
	responseTools := make([]openAIResponsesTool, 0, len(registeredTools))
	for _, tool := range registeredTools {
		responseTools = append(responseTools, openAIResponsesTool{
			Type:        "function",
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		})
	}
	return responseTools
}

func buildOpenAIResponseRequest(req GenReqJsonPvRequest, registeredTools []tools.ToolReg) (OpenaiResponse, []MemoryMessage) {
	memory := make([]MemoryMessage, 0, len(req.Memory)+2)
	memory = append(memory, req.Memory...)
	if len(req.Memory) == 0 && req.SystemPrompt != "" {
		memory = append(memory, MemoryMessage{Role: "system", Content: req.SystemPrompt})
	}
	if req.UserInput != "" {
		memory = append(memory, MemoryMessage{Role: "user", Content: req.UserInput})
	}
	systemRole := ""
	if req.Provider == "oauth:codex" {
		systemRole = "developer"
	}
	request := OpenaiResponse{
		Model:        req.Model,
		Instructions: req.SystemPrompt,
		Input:        buildOpenAIResponsesInput(req.UserInput, req.Memory, systemRole),
		Stream:       true,
		Text: openAIResponsesText{
			Format: openAIResponsesTextFormat{Type: "text"},
		},
	}
	if req.Provider == "oauth:codex" {
		store := false
		parallelToolCalls := true
		request.Store = &store
		request.ParallelToolCalls = &parallelToolCalls
		request.Include = []string{"reasoning.encrypted_content"}
	} else {
		maxOutputTokens := 4096
		temperature := 1.0
		topP := 1.0
		request.MaxOutputTokens = &maxOutputTokens
		request.Temperature = &temperature
		request.TopP = &topP
	}
	if req.ThinkLevel != "" {
		request.Reasoning = &openAIResponsesReasoning{Effort: req.ThinkLevel}
	}
	if len(registeredTools) > 0 {
		request.Tools = buildOpenAIResponsesTools(registeredTools)
		request.ToolChoice = "auto"
	}
	return request, memory
}
