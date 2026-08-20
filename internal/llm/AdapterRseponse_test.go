package llm

import (
	"encoding/json"
	"testing"
)

func TestGenReqJsonPvResponsesStreamMatchesTransport(t *testing.T) {
	for _, test := range []struct {
		provider string
		stream   bool
	}{
		{provider: "openai:responses", stream: false},
		{provider: "oauth:codex", stream: true},
	} {
		t.Run(test.provider, func(t *testing.T) {
			_, requestJSON, err := GenReqJsonPv(GenReqJsonPvRequest{Provider: test.provider, Model: "test-model"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]any
			if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
				t.Fatal(err)
			}
			if request["stream"] != test.stream {
				t.Fatalf("stream = %#v, want %v", request["stream"], test.stream)
			}
		})
	}
}

func TestGenReqJsonPvResponsesRequestsReasoningSummary(t *testing.T) {
	for _, provider := range []string{"openai:responses", "oauth:codex"} {
		t.Run(provider, func(t *testing.T) {
			_, requestJSON, err := GenReqJsonPv(GenReqJsonPvRequest{
				Provider:   provider,
				Model:      "test-model",
				ThinkLevel: "medium",
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]any
			if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
				t.Fatal(err)
			}
			reasoning, ok := request["reasoning"].(map[string]any)
			if !ok || reasoning["effort"] != "medium" || reasoning["summary"] != "auto" {
				t.Fatalf("reasoning request = %#v", request["reasoning"])
			}
			if provider == "oauth:codex" {
				include, ok := request["include"].([]any)
				if !ok || len(include) != 1 || include[0] != "reasoning.encrypted_content" {
					t.Fatalf("include = %#v", request["include"])
				}
			}
		})
	}
}
