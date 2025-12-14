package google

import (
	"context"
	"fmt"
	"io"

	"github.com/bduffany/gpt-cli/internal/llm"
	"google.golang.org/genai"
)

func GetDefaultModel(thinking bool) string {
	if thinking {
		return "gemini-3-pro-preview"
	} else {
		return "gemini-2.5-flash"
	}
}

type GeminiClient struct {
	ModelName string
	client    *genai.Client
}

var _ llm.CompletionClient = (*GeminiClient)(nil)

func NewGeminiClient(model string) (*GeminiClient, error) {
	client, err := genai.NewClient(context.TODO(), nil)
	if err != nil {
		return nil, err
	}
	return &GeminiClient{
		ModelName: model,
		client:    client,
	}, nil
}

func (c *GeminiClient) GetCompletion(ctx context.Context, messages []llm.Message) (*llm.Completion, error) {
	var parts []*genai.Content
	var systemInstruction *genai.Content
	for _, m := range messages {
		if m.Metadata.Role == llm.RoleSystem {
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: m.Payload}},
			}
			continue
		}
		parts = append(parts, &genai.Content{
			Role:  string(convertRole(m.Metadata.Role)),
			Parts: []*genai.Part{{Text: m.Payload}},
		})
	}
	stream := c.client.Models.GenerateContentStream(ctx, c.ModelName, parts, &genai.GenerateContentConfig{
		SystemInstruction: systemInstruction,
		CandidateCount:    1,
	})
	pr, pw := io.Pipe()
	go func() (err error) {
		defer func() { pw.CloseWithError(err) }()
		for rsp, err := range stream {
			if err != nil {
				return fmt.Errorf("generate content: %w", err)
			}
			_, err = pw.Write([]byte(rsp.Text()))
			if err != nil {
				return fmt.Errorf("write content to pipe: %w", err)
			}
		}
		return nil
	}()

	return &llm.Completion{ReadCloser: pr}, nil
}

func convertRole(r llm.Role) genai.Role {
	switch r {
	case llm.RoleUser:
		return genai.RoleUser
	case llm.RoleModel:
		return genai.RoleModel
	default:
		panic(fmt.Sprintf("unknown role: %v", r))
	}
}
