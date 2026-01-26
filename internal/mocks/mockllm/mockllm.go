package mockllm

import (
	"context"
	"io"
	"strings"

	"github.com/bduffany/gpt-cli/internal/llm"
)

// Client is a test implementation of llm.CompletionClient that returns
// predefined responses.
type Client struct {
	// Response is the fixed response to return for all completions.
	Response string

	// ResponseFunc, if set, is called to generate the response.
	// It takes precedence over Response.
	ResponseFunc func(messages []llm.Message) string

	// ReceivedMessages stores the messages from the last GetCompletion call.
	ReceivedMessages []llm.Message
}

var _ llm.CompletionClient = (*Client)(nil)

func (m *Client) GetCompletion(ctx context.Context, messages []llm.Message) (*llm.Completion, error) {
	m.ReceivedMessages = messages

	response := m.Response
	if m.ResponseFunc != nil {
		response = m.ResponseFunc(messages)
	}

	return &llm.Completion{
		ReadCloser: io.NopCloser(strings.NewReader(response)),
	}, nil
}
