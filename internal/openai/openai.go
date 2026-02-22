package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/bduffany/gpt-cli/internal/llm"
)

const (
	DefaultModel                 = "gpt-4.1"
	DefaultVerifiedModel         = "gpt-5.2"
	DefaultThinkingModel         = "o1"
	DefaultVerifiedThinkingModel = "gpt-5.2?reasoning_effort=xhigh"
)

// ModelParams contains parameters parsed from a model string.
type ModelParams struct {
	ReasoningEffort string
}

// ParseModel parses a model string like "gpt-5.2?reasoning_effort=high"
// and returns the model name and any parameters.
func ParseModel(modelString string) (name string, params ModelParams) {
	idx := strings.Index(modelString, "?")
	if idx == -1 {
		return modelString, ModelParams{}
	}

	name = modelString[:idx]
	queryString := modelString[idx+1:]

	values, err := url.ParseQuery(queryString)
	if err != nil {
		// If parsing fails, return the original string as the model name
		return modelString, ModelParams{}
	}

	params.ReasoningEffort = values.Get("reasoning_effort")
	return name, params
}

func GetDefaultModel(thinking bool) string {
	verifiedEnvVar := strings.ToLower(strings.TrimSpace(os.Getenv("OPENAI_IDENTITY_VERIFIED")))
	verified := verifiedEnvVar == "1" || verifiedEnvVar == "true" || verifiedEnvVar == "yes"
	if verified {
		if thinking {
			return DefaultVerifiedThinkingModel
		} else {
			return DefaultVerifiedModel
		}
	} else {
		if thinking {
			return DefaultThinkingModel
		} else {
			return DefaultModel
		}
	}
}

type Client struct {
	ModelName string
	Token     string

	ReasoningEffort string
}

var _ llm.CompletionClient = (*Client)(nil)

func (c *Client) GetCompletion(ctx context.Context, messages []llm.Message) (*llm.Completion, error) {
	payload := map[string]any{
		"model":  c.ModelName,
		"stream": true,
		"input":  getResponseInput(messages),
	}
	if c.ReasoningEffort != "" {
		payload["reasoning"] = map[string]string{
			"effort": c.ReasoningEffort,
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	rsp, err := c.Request(ctx, "POST", "/v1/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Return a pipe reader with the parsed streamed response text.
	pr, pw := io.Pipe()
	go func() (err error) {
		defer rsp.Body.Close()
		defer func() { pw.CloseWithError(err) }()
		return writeResponseTextStream(rsp.Body, pw)
	}()

	return &llm.Completion{ReadCloser: pr}, nil
}

func (c *Client) GetJSON(ctx context.Context, endpoint string, obj any) error {
	rsp, err := c.Request(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	b, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(b, obj); err != nil {
		return err
	}
	return nil
}

func (c *Client) Request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "https://api.openai.com"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode >= 300 {
		defer rsp.Body.Close()
		b, err := io.ReadAll(rsp.Body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %d (got response header but reading response body failed with: %s)", rsp.StatusCode, err)
		}
		e := &ErrorResponse{}
		if err := json.Unmarshal(b, e); err != nil {
			return nil, fmt.Errorf("HTTP %d: server reply: %q", rsp.StatusCode, string(b))
		}
		if e.Error == nil {
			return nil, fmt.Errorf("HTTP %d: server reply: %q", rsp.StatusCode, string(b))
		}
		return nil, e.Error
	}

	return rsp, nil
}

func writeResponseTextStream(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// Use a larger buffer for long JSON lines in SSE events.
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
			break
		}
		event := &ResponseEvent{}
		if err := json.Unmarshal([]byte(data), event); err != nil {
			return fmt.Errorf("failed to parse response event %q: %w", line, err)
		}
		if event.Type == "response.output_text.delta" {
			if _, err := io.WriteString(w, event.Delta); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func getResponseInput(messages []llm.Message) []ResponseInputItem {
	items := make([]ResponseInputItem, len(messages))
	for i, m := range messages {
		items[i] = ResponseInputItem{
			Role:    convertRole(m.Metadata.Role),
			Content: m.Payload,
		}
	}
	return items
}

func convertRole(r llm.Role) string {
	switch r {
	case llm.RoleUser:
		return "user"
	case llm.RoleModel:
		return "assistant"
	case llm.RoleSystem:
		return "system"
	default:
		panic(fmt.Sprintf("unknown role: %v", r))
	}
}

// OpenAPI spec

type OpenAPISpec struct {
	Components struct {
		Schemas struct {
			AssistantSupportedModels struct {
				Enum []string `yaml:"enum"`
			} `yaml:"AssistantSupportedModels"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

// Models API definitions

type ListModelsResponse struct {
	Data []Model `json:"data"`
}

type Model struct {
	ID string `json:"id"`
}

// Assistants API definitions

type ListAssistantsResponse struct {
	Data []AssistantObject `json:"data"`
}

type AssistantObject struct {
	ID string `json:"id"`
}

// Responses API definitions

type ResponseInputItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
}

// Common API definitions

type GenericObject struct {
	// "list" | "model"
	Object string `json:"object"`
	// TODO: should be any?
	Data    []GenericObject `json:"data"`
	ID      string          `json:"id"`
	Created int64           `json:"created"`
	OwnedBy string          `json:"owned_by"`
}

type ErrorResponse struct {
	Error *Error `json:"error,omitempty"`
}

type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   any    `json:"param"`
	Code    any    `json:"code"`
}

func (a *Error) Error() string {
	return fmt.Sprintf("%s: %s", a.Type, a.Message)
}
