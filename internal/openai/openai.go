package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bduffany/gpt-cli/internal/llm"
)

const (
	DefaultModel                 = "gpt-4.1"
	DefaultVerifiedModel         = "gpt-5.1"
	DefaultThinkingModel         = "o1"
	DefaultVerifiedThinkingModel = "o3"
)

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
}

var _ llm.CompletionClient = (*Client)(nil)

func (c *Client) GetCompletion(ctx context.Context, messages []llm.Message) (*llm.Completion, error) {
	payload := map[string]any{
		"model":    c.ModelName,
		"stream":   true,
		"messages": getOpenAIMessages(messages),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	rsp, err := c.Request(ctx, "POST", "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Return a pipe reader with the parsed chat response.
	pr, pw := io.Pipe()
	go func() (err error) {
		defer rsp.Body.Close()
		defer func() { pw.CloseWithError(err) }()

		reply := &bytes.Buffer{}

		w := io.MultiWriter(pw, reply)

		scanner := bufio.NewScanner(rsp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) < 2 {
				continue
			}
			if parts[0] != "data" {
				continue
			}
			if parts[1] == "[DONE]" {
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
				break
			}
			data := &Data{}
			if err := json.Unmarshal([]byte(parts[1]), data); err != nil {
				return fmt.Errorf("failed to parse line %q: %s", line, err)
			}
			// TODO: nil checks
			if _, err := io.WriteString(w, data.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
		if scanner.Err() != nil {
			return scanner.Err()
		}
		return nil
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
	req.Header.Set("OpenAI-Beta", "assistants=v2")
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

func getOpenAIMessages(messages []llm.Message) []Message {
	openaiMessages := make([]Message, len(messages))
	for i, m := range messages {
		openaiMessages[i] = Message{
			Role:    convertRole(m.Metadata.Role),
			Content: m.Payload,
		}
	}
	return openaiMessages
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

// Completions API definitions

type Message struct {
	// "system" | "user"
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Data struct {
	Choices []*Choice
}

type Choice struct {
	Delta *Delta
}

type Delta struct {
	Content string
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
