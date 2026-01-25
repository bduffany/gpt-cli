package openai

import "testing"

func TestParseModel(t *testing.T) {
	tests := []struct {
		input           string
		wantName        string
		wantEffort      string
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
