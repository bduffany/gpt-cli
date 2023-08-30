package main

import (
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
	model      = flag.String("model", "gpt-4-1106-preview", "`gpt-*` model to use")
	listModels = flag.Bool("models", false, "List available models and exit.")

	autoMode = flag.Bool("auto", false, "Function as a fully automated assistant, with access to tools.")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	token := os.Getenv("OPENAI_API_KEY")
	if token == "" {
		return fmt.Errorf("missing OPENAI_API_KEY env var")
	}
	client := &api.Client{Token: token}
	if *listModels {
		return printAvailableModels(client)
	}

	c, err := chat.New(client)
	if err != nil {
		return err
	}
	c.Model = *model
	if *autoMode {
		return auto.Run(c)
	}
	if err := c.Run(); err != nil {
		return err
	}
	return nil
}

func printAvailableModels(c *api.Client) error {
	rsp := &api.GenericObject{}
	if err := c.GetJSON("/v1/models", rsp); err != nil {
		return err
	}
	for _, obj := range rsp.Data {
		if strings.HasPrefix(obj.ID, "gpt-") {
			fmt.Println(obj.ID)
		}
	}
	return nil
}
