package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bduffany/gpt-cli/internal/api"
	"github.com/bduffany/gpt-cli/internal/auto"
	"github.com/bduffany/gpt-cli/internal/chat"

	_ "embed"
)

var (
	model      = flag.String("model", "gpt-4o-2024-08-06", "`gpt-*` model to use")
	listModels = flag.Bool("models", false, "List available models and exit.")

	systemPrompt = flag.String("system", "You are a helpful assistant.", "System prompt.")
	promptFile   = flag.String("prompt_file", "", "Load prompt from a file at this path. If unset, read from stdin.")
	interactive  = flag.Bool("interactive", false, "Start an interactive session even after loading prompt_file or reading the prompt from args. stdin must be a terminal.")

	autoMode = flag.Bool("auto", false, "Function as a fully automated assistant, with access to tools.")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	ctx := context.Background()

	token := os.Getenv("OPENAI_API_KEY")
	if token == "" {
		return fmt.Errorf("missing OPENAI_API_KEY env var")
	}
	client := &api.Client{Token: token}
	if *listModels {
		return printAvailableModels(ctx, client)
	}

	// TODO: allow loading messages from a previous session
	var messages []api.Message
	if *systemPrompt != "" {
		messages = append(messages, api.Message{
			Role:    "system",
			Content: *systemPrompt,
		})
	}
	c, err := chat.New(client, messages)
	if err != nil {
		return err
	}
	c.Model = *model
	if *autoMode {
		return auto.Run(ctx, c)
	}

	promptFromArgs := strings.Join(flag.Args(), " ")
	if *promptFile != "" {
		f, err := os.Open(*promptFile)
		if err != nil {
			return fmt.Errorf("open %s: %w", *promptFile, err)
		}
		defer f.Close()
		c.PromptReader = f
		c.Interactive = *interactive
	} else if promptFromArgs != "" {
		c.PromptReader = strings.NewReader(promptFromArgs)
		c.Interactive = *interactive
	}
	if err := c.Run(ctx); err != nil {
		return err
	}
	return nil
}

func printAvailableModels(ctx context.Context, c *api.Client) error {
	rsp := &api.GenericObject{}
	if err := c.GetJSON(ctx, "/v1/models", rsp); err != nil {
		return err
	}
	for _, obj := range rsp.Data {
		if strings.HasPrefix(obj.ID, "gpt-") {
			fmt.Println(obj.ID)
		}
	}
	return nil
}
