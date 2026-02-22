package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/bduffany/gpt-cli/internal/agent"
	"github.com/bduffany/gpt-cli/internal/chat"
	"github.com/bduffany/gpt-cli/internal/flags"
	"github.com/bduffany/gpt-cli/internal/google"
	"github.com/bduffany/gpt-cli/internal/llm"
	"github.com/bduffany/gpt-cli/internal/openai"
	"github.com/bduffany/gpt-cli/internal/persona"
	"gopkg.in/yaml.v3"

	_ "embed"
)

var (
	fs = flags.NewFlagSet()

	listModels    = fs.Bool("models", false, "List available models and exit.")
	listAllModels = fs.Bool("all-models", false, "List ALL models and exit, even ones that aren't specified in AssistantSupportedModels.")

	model    = fs.String("model", "", "gpt-* or gemini-* model to use.")
	gemini   = fs.Bool("gemini", false, "Use Gemini (takes precedence over --model).", flags.Short("g"))
	thinking = fs.Bool("thinking", false, "Use a thinking model (Gemini pro or OpenAI o1/o3).", flags.Short("t"))
	five     = fs.Bool("5", false, "Shorthand for --model=gpt-5.")
	effort   = fs.String("effort", "", "Sets the reasoning effort parameter for models that support it.", flags.Short("e"))

	systemPrompt = fs.String("system", "", "System prompt. Defaults to a prompt containing basic OS and session info.")
	promptFile   = fs.String("prompt-file", "", "Load prompt from a file at this path. If unset, read from stdin.")
	interactive  = fs.Bool("interactive", false, "Start an interactive session even after loading prompt-file or reading the prompt from args. stdin must be a terminal.", flags.Short("i"))

	agentMode = fs.Bool("agent", false, "Function as a fully automated agent, with access to tools.")

	personaFlag = fs.String("persona", "", "Persona name or path.", flags.Short("p"))
)

func main() {
	if err := run(); err != nil {
		if err == flags.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "personas" {
		return runPersonasCommand(os.Args[2:])
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

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

	// Load persona: flag takes precedence, then AI_PERSONA env var
	personaName := *personaFlag
	personaSource := "-p/--persona flag"
	if personaName == "" {
		personaName = os.Getenv("AI_PERSONA")
		personaSource = "AI_PERSONA env var"
	}
	personaText, err := persona.Load(personaName)
	if err != nil {
		return fmt.Errorf("load persona (via %s): %w", personaSource, err)
	}

	// Build system prompt
	if *systemPrompt == "" {
		*systemPrompt = getDefaultSystemPrompt()
	}
	if personaText != "" {
		*systemPrompt = *systemPrompt + "\n\n" + personaText
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
		// Parse model string for embedded parameters (e.g., "gpt-5.2?reasoning_effort=high")
		modelName, modelParams := openai.ParseModel(*model)
		reasoningEffort := *effort
		// Model string params are used as defaults; explicit flag takes precedence
		if reasoningEffort == "" {
			reasoningEffort = modelParams.ReasoningEffort
		}
		openAIClient := &openai.Client{
			ModelName:       modelName,
			ReasoningEffort: reasoningEffort,
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
		return agent.Run(ctx, c)
	}

	promptFromArgs := strings.Join(fs.Args(), " ")
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

func runPersonasCommand(args []string) error {
	subcommand, dir, all, err := parsePersonasArgs(args)
	if err != nil {
		if err == flags.ErrHelp {
			return err
		}
		return err
	}

	personas, err := collectPersonas(all)
	if err != nil {
		return err
	}

	switch subcommand {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(personas)
	case "export":
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create export dir %s: %w", dir, err)
		}
		for name, desc := range personas {
			path := filepath.Join(dir, name+".txt")
			if !strings.HasSuffix(desc, "\n") {
				desc += "\n"
			}
			if err := os.WriteFile(path, []byte(desc), 0644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown personas subcommand: %s", subcommand)
	}
}

func parsePersonasArgs(args []string) (subcommand, dir string, all bool, err error) {
	if len(args) == 0 {
		return "json", "", false, nil
	}

	var positional []string
	for _, arg := range args {
		switch arg {
		case "-a", "--all":
			all = true
		case "-h", "--help":
			printPersonasUsage()
			return "", "", false, flags.ErrHelp
		default:
			if strings.HasPrefix(arg, "-") {
				return "", "", false, fmt.Errorf("unknown flag: %s", arg)
			}
			positional = append(positional, arg)
		}
	}

	subcommand = "json"
	if len(positional) > 0 {
		subcommand = positional[0]
		positional = positional[1:]
	}

	switch subcommand {
	case "json":
		if len(positional) != 0 {
			return "", "", false, fmt.Errorf("usage: gpt personas [json] [-a|--all]")
		}
	case "export":
		if len(positional) != 1 {
			return "", "", false, fmt.Errorf("usage: gpt personas export [-a|--all] <dir>")
		}
		dir = positional[0]
	default:
		return "", "", false, fmt.Errorf("unknown personas subcommand: %s", subcommand)
	}

	return subcommand, dir, all, nil
}

func printPersonasUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gpt personas [json] [-a|--all]")
	fmt.Fprintln(os.Stderr, "  gpt personas export [-a|--all] <dir>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -a, --all   include user personas (overriding built-ins)")
}

func collectPersonas(includeAll bool) (map[string]string, error) {
	personas := persona.BuiltinPersonasMap()
	if !includeAll {
		return personas, nil
	}
	userPersonas, err := persona.LoadUserPersonas()
	if err != nil {
		return nil, err
	}
	for name, desc := range userPersonas {
		personas[name] = desc
	}
	return personas, nil
}

func getDefaultSystemPrompt() string {
	lines := []string{
		"You are a helpful AI chat assistant being accessed through a command line tool.",
		"Your underlying AI model name/version is: " + *model,
		"The chat session started at " + time.Now().String() + " local time.",
		"The host OS is " + fmt.Sprintf("%s (%s)", runtime.GOOS, runtime.GOARCH) + ".",
		"The output display does NOT support markdown or MathJax rendering.",
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		lines = append(lines, "The user's shell is "+shell+".")
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
