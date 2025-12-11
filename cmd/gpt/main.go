package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

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

	model    = flag.String("model", "", "`gpt-* or gemini-*` model to use.")
	gemini   = flag.Bool("g", false, "Use Gemini (takes precedence over -model)")
	thinking = flag.Bool("t", false, "Use a thinking model (Gemini pro or OpenAI o1/o3).")
	five     = flag.Bool("5", false, "Shorthand for -model=gpt-5.")
	effort   = flag.String("effort", "", "Sets the reasoning effort parameter for models that support it.")

	systemPrompt = flag.String("system", "", "System prompt. Defaults to a prompt containing basic OS and session info.")
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

	if *five {
		*model = "gpt-5"
	}
	if *model == "" {
		if *gemini {
			*model = google.GetDefaultModel(*thinking)
		} else {
			*model = openai.GetDefaultModel(*thinking)
		}
	}

	if *systemPrompt == "" {
		*systemPrompt = getDefaultSystemPrompt()
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
			ModelName:       *model,
			ReasoningEffort: *effort,
			Token:           token,
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

func getDefaultSystemPrompt() string {
	lines := []string{
		"You are a helpful AI chat assistant being accessed through a command line tool.",
		"Your underlying AI model name/version is: " + *model,
		"The chat session started at " + time.Now().String() + " local time.",
		"The host OS is " + fmt.Sprintf("%s (%s)", runtime.GOOS, runtime.GOARCH) + ".",
	}
	if runtime.GOOS == "linux" {
		// Read /etc/os-release and look for PRETTY_NAME
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					lines = append(lines, "The host Linux distribution is "+strings.Trim(line[len("PRETTY_NAME="):], `"`)+".")
					break
				}
			}
		}
	}
	return strings.Join(lines, "\n")
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
