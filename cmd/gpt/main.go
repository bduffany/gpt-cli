package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/bduffany/gpt-cli/internal/auto"
	"github.com/bduffany/gpt-cli/internal/chat"
	"github.com/bduffany/gpt-cli/internal/google"
	"github.com/bduffany/gpt-cli/internal/llm"
	"github.com/bduffany/gpt-cli/internal/openai"
	"gopkg.in/yaml.v3"

	_ "embed"
)

var (
	listModels    = flag.Bool("models", false, "List available models and exit.")
	listAllModels = flag.Bool("all_models", false, "List ALL models and exit, even ones that aren't specified in AssistantSupportedModels.")

	model    = flag.String("model", openai.DefaultModel, "`gpt-* or gemini-*` model to use.")
	gemini   = flag.Bool("g", false, "Use Gemini (takes precedence over -model)")
	thinking = flag.Bool("t", false, "Use a thinking model (Gemini pro or OpenAI o1).")

	systemPrompt = flag.String("system", "You are a helpful assistant.", "System prompt.")
	promptFile   = flag.String("prompt_file", "", "Load prompt from a file at this path. If unset, read from stdin.")
	interactive  = flag.Bool("interactive", false, "Start an interactive session even after loading prompt_file or reading the prompt from args. stdin must be a terminal.")

	agentMode = flag.Bool("agent", false, "Function as a fully automated agent, with access to tools.")
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

	var client llm.CompletionClient

	if *gemini {
		if *thinking {
			*model = "gemini-2.5-pro"
		} else {
			*model = "gemini-2.5-flash"
		}
	} else if *thinking {
		*model = "o1"
	}

	if isGeminiModel(*model) {
		gem, err := google.NewGeminiClient(*model)
		if err != nil {
			return fmt.Errorf("create Gemini client: %w", err)
		}
		client = gem
	} else {
		// Assume OpenAI for now.
		token := os.Getenv("OPENAI_API_KEY")
		if token == "" {
			return fmt.Errorf("missing OPENAI_API_KEY env var")
		}
		if *listModels {
			return printAssistantSupportedModels(ctx)
		}
		openAIClient := &openai.Client{
			ModelName: *model,
			Token:     token,
		}
		if *listAllModels {
			return printAvailableModels(ctx, openAIClient)
		}
		client = openAIClient
	}

	// TODO: allow loading messages from a previous session
	var messages []llm.Message
	if *systemPrompt != "" {
		messages = append(messages, llm.Message{
			Metadata: llm.MessageMetadata{
				Role: llm.RoleSystem,
			},
			Payload: *systemPrompt,
		})
	}
	c, err := chat.New(client, messages)
	if err != nil {
		return err
	}
	c.Model = *model
	if *agentMode {
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

func isGeminiModel(model string) bool {
	return strings.HasPrefix(model, "gemini-")
}

func printAvailableModels(ctx context.Context, client *openai.Client) error {
	models := &openai.ListModelsResponse{}
	if err := client.GetJSON(ctx, "/v1/models", models); err != nil {
		return fmt.Errorf("list models: %w", err)
	}
	var ids []string
	for _, m := range models.Data {
		ids = append(ids, m.ID)
	}
	slices.Sort(ids)
	for _, id := range ids {
		fmt.Println(id)
	}
	return nil
}

func printAssistantSupportedModels(ctx context.Context) error {

	// Note: /v1/models API doesn't filter to chat-only models,
	// so we use the OpenAPI spec.

	// Hopefully this URL remains stable :P
	const specURL = "https://raw.githubusercontent.com/openai/openai-openapi/refs/heads/master/openapi.yaml"

	// Fetch the spec
	req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
	if err != nil {
		return fmt.Errorf("create GET request for %s: %w", specURL, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", specURL, err)
	}
	defer resp.Body.Close()

	// Parse response as YAML
	var spec openai.OpenAPISpec
	if err := yaml.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return fmt.Errorf("decode %s: %w", specURL, err)
	}

	models := spec.Components.Schemas.AssistantSupportedModels.Enum
	for _, model := range models {
		fmt.Println(model)
	}

	return nil
}
