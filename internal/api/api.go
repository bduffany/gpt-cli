package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	DefaultModel = "gpt-4o"
)

type Client struct {
	Token string
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
			return nil, fmt.Errorf("HTTP %d, body_read_error=%s", rsp.StatusCode, err)
		}
		e := &ErrorResponse{}
		if err := json.Unmarshal(b, e); err != nil {
			return nil, fmt.Errorf("HTTP %d, body=%q", rsp.StatusCode, string(b))
		}
		if e.Error == nil {
			return nil, fmt.Errorf("HTTP %d, body=%q", rsp.StatusCode, string(b))
		}
		return nil, e.Error
	}

	return rsp, nil
}

// Completions API definitions

type Message struct {
	// "system" | "user"
	Role    string `json:"role,omitEmpty"`
	Content string `json:"content,omitEmpty"`
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
	Error *Error `json:"error,omitEmpty"`
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
