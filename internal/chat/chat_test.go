package chat

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bduffany/gpt-cli/internal/llm"
	"github.com/bduffany/gpt-cli/internal/mocks/mockllm"
)

func TestChatBasicExchange(t *testing.T) {
	// Create a mock client that returns a fixed response
	mockClient := &mockllm.Client{
		Response: "Hello! How can I help you today?",
	}

	// Create chat with the mock client
	c, err := New(mockClient, nil)
	if err != nil {
		t.Fatalf("failed to create chat: %v", err)
	}

	// Wire up stdin/stdout to test buffers
	c.PromptReader = strings.NewReader("Hello")
	c.Interactive = false

	var stdout bytes.Buffer
	c.Display = &stdout

	// Run the chat
	ctx := context.Background()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("chat.Run failed: %v", err)
	}

	// Verify the output contains the expected response
	output := stdout.String()
	expectedResponse := "Hello! How can I help you today?"
	if !strings.Contains(output, expectedResponse) {
		t.Errorf("expected output to contain %q, got: %q", expectedResponse, output)
	}

	// Verify the mock received the user's message
	if len(mockClient.ReceivedMessages) == 0 {
		t.Fatal("expected mock client to receive messages")
	}

	lastMsg := mockClient.ReceivedMessages[len(mockClient.ReceivedMessages)-1]
	if lastMsg.Metadata.Role != llm.RoleUser {
		t.Errorf("expected last message role to be user, got: %v", lastMsg.Metadata.Role)
	}
	if lastMsg.Payload != "Hello" {
		t.Errorf("expected last message payload to be 'Hello', got: %q", lastMsg.Payload)
	}
}

func TestChatWithSystemPrompt(t *testing.T) {
	mockClient := &mockllm.Client{
		Response: "I'm doing great, thanks for asking!",
	}

	// Create chat with a system prompt
	systemMessages := []llm.Message{
		{
			Metadata: llm.MessageMetadata{Role: llm.RoleSystem},
			Payload:  "You are a friendly assistant.",
		},
	}

	c, err := New(mockClient, systemMessages)
	if err != nil {
		t.Fatalf("failed to create chat: %v", err)
	}

	c.PromptReader = strings.NewReader("How are you?")
	c.Interactive = false

	var stdout bytes.Buffer
	c.Display = &stdout

	ctx := context.Background()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("chat.Run failed: %v", err)
	}

	// Verify the mock received both system and user messages
	if len(mockClient.ReceivedMessages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(mockClient.ReceivedMessages))
	}

	if mockClient.ReceivedMessages[0].Metadata.Role != llm.RoleSystem {
		t.Errorf("expected first message to be system role")
	}
	if mockClient.ReceivedMessages[0].Payload != "You are a friendly assistant." {
		t.Errorf("expected system prompt, got: %q", mockClient.ReceivedMessages[0].Payload)
	}

	if mockClient.ReceivedMessages[1].Metadata.Role != llm.RoleUser {
		t.Errorf("expected second message to be user role")
	}
	if mockClient.ReceivedMessages[1].Payload != "How are you?" {
		t.Errorf("expected user prompt, got: %q", mockClient.ReceivedMessages[1].Payload)
	}
}

func TestChatAccumulatesHistory(t *testing.T) {
	mockClient := &mockllm.Client{
		Response: "Nice to meet you!",
	}

	c, err := New(mockClient, nil)
	if err != nil {
		t.Fatalf("failed to create chat: %v", err)
	}

	c.PromptReader = strings.NewReader("Hi there")
	c.Interactive = false

	var stdout bytes.Buffer
	c.Display = &stdout

	ctx := context.Background()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("chat.Run failed: %v", err)
	}

	// Verify conversation history accumulated (user message + model response)
	if len(c.Messages) != 2 {
		t.Fatalf("expected 2 messages in history (user + model), got %d", len(c.Messages))
	}

	if c.Messages[0].Metadata.Role != llm.RoleUser {
		t.Errorf("expected first message to be user role")
	}
	if c.Messages[0].Payload != "Hi there" {
		t.Errorf("expected user message 'Hi there', got: %q", c.Messages[0].Payload)
	}

	if c.Messages[1].Metadata.Role != llm.RoleModel {
		t.Errorf("expected second message to be model role")
	}
	if c.Messages[1].Payload != "Nice to meet you!" {
		t.Errorf("expected model response 'Nice to meet you!', got: %q", c.Messages[1].Payload)
	}
}

func TestChatEmptyInput(t *testing.T) {
	mockClient := &mockllm.Client{
		Response: "You didn't say anything!",
	}

	c, err := New(mockClient, nil)
	if err != nil {
		t.Fatalf("failed to create chat: %v", err)
	}

	c.PromptReader = strings.NewReader("")
	c.Interactive = false

	var stdout bytes.Buffer
	c.Display = &stdout

	ctx := context.Background()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("chat.Run failed: %v", err)
	}

	// Even empty input should get a response
	if !strings.Contains(stdout.String(), "You didn't say anything!") {
		t.Errorf("expected response for empty input, got: %q", stdout.String())
	}
}
