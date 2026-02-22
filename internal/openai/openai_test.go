package openai

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bduffany/gpt-cli/internal/llm"
)

func TestParseModel(t *testing.T) {
	tests := []struct {
		input      string
		wantName   string
		wantEffort string
	}{
		{
			input:    "gpt-4.1",
			wantName: "gpt-4.1",
		},
		{
			input:      "gpt-5.2?reasoning_effort=high",
			wantName:   "gpt-5.2",
			wantEffort: "high",
		},
		{
			input:      "gpt-5.2?reasoning_effort=xhigh",
			wantName:   "gpt-5.2",
			wantEffort: "xhigh",
		},
		{
			input:      "o3?reasoning_effort=low",
			wantName:   "o3",
			wantEffort: "low",
		},
		{
			input:    "gpt-5.2?other_param=value",
			wantName: "gpt-5.2",
		},
		{
			input:      "gpt-5.2?reasoning_effort=medium&other=value",
			wantName:   "gpt-5.2",
			wantEffort: "medium",
		},
		{
			input:    "",
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, params := ParseModel(tt.input)
			if name != tt.wantName {
				t.Errorf("ParseModel(%q) name = %q, want %q", tt.input, name, tt.wantName)
			}
			if params.ReasoningEffort != tt.wantEffort {
				t.Errorf("ParseModel(%q) ReasoningEffort = %q, want %q", tt.input, params.ReasoningEffort, tt.wantEffort)
			}
		})
	}
}

func TestGetDefaultModel(t *testing.T) {
	// Just verify it returns non-empty strings
	model := GetDefaultModel(false)
	if model == "" {
		t.Error("GetDefaultModel(false) returned empty string")
	}

	thinkingModel := GetDefaultModel(true)
	if thinkingModel == "" {
		t.Error("GetDefaultModel(true) returned empty string")
	}
}

func TestWriteResponseTextStream(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created"}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		`data: {"type":"response.output_text.delta","delta":" world"}`,
		`data: {"type":"response.completed"}`,
		"data: [DONE]",
		"",
	}, "\n"))

	var out bytes.Buffer
	if err := writeResponseTextStream(stream, &out); err != nil {
		t.Fatalf("writeResponseTextStream: %v", err)
	}

	if got, want := out.String(), "Hello world\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestGetResponseInput(t *testing.T) {
	messages := []llm.Message{
		{Metadata: llm.MessageMetadata{Role: llm.RoleSystem}, Payload: "You are helpful."},
		{Metadata: llm.MessageMetadata{Role: llm.RoleUser}, Payload: "Hi"},
		{Metadata: llm.MessageMetadata{Role: llm.RoleModel}, Payload: "Hello"},
	}

	input := getResponseInput(messages)
	if len(input) != len(messages) {
		t.Fatalf("len(input) = %d, want %d", len(input), len(messages))
	}

	tests := []struct {
		index int
		role  string
		text  string
	}{
		{index: 0, role: "system", text: "You are helpful."},
		{index: 1, role: "user", text: "Hi"},
		{index: 2, role: "assistant", text: "Hello"},
	}
	for _, tt := range tests {
		item := input[tt.index]
		if item.Role != tt.role {
			t.Fatalf("input[%d].Role = %q, want %q", tt.index, item.Role, tt.role)
		}
		if item.Content != tt.text {
			t.Fatalf("input[%d].Content = %q, want %q", tt.index, item.Content, tt.text)
		}
	}
}
