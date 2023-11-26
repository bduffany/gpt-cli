package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/bduffany/gpt-cli/internal/api"
	"github.com/chzyer/readline"
	"github.com/mattn/go-isatty"
)

const (
	defaultModel        = "gpt-4"
	defaultSystemPrompt = "You are a helpful assistant."
)

type Chat struct {
	Interactive  bool
	Model        string
	SystemPrompt string
	PromptFile   string
	History      []api.Message

	Display io.Writer

	client   *api.Client
	readline *readline.Instance
}

func New(client *api.Client) (*Chat, error) {
	var rl *readline.Instance
	interactive := isatty.IsTerminal(os.Stdin.Fd())
	return &Chat{
		client:       client,
		readline:     rl,
		Display:      os.Stdout,
		SystemPrompt: defaultSystemPrompt,
		Model:        defaultModel,
		Interactive:  interactive,
	}, nil
}

func (c *Chat) GetPrompt() (string, error) {
	if c.PromptFile != "" && len(c.History) == 0 {
		b, err := os.ReadFile(c.PromptFile)
		return string(b), err
	} else if c.PromptFile != "" && !c.Interactive {
		return "", io.EOF
	}

	if c.Interactive && c.readline == nil {
		r, err := readline.New(Esc(90) + "you> " + Esc())
		if err != nil {
			return "", err
		}
		c.readline = r
	}

	if c.readline != nil {
		return c.readline.Readline()
	}

	b, err := io.ReadAll(os.Stdin)
	return string(b), err
}

func (c *Chat) Confirmf(format string, args ...any) (bool, string, error) {
	io.WriteString(c.Display, Esc(93)+fmt.Sprintf(format, args...)+" (yes / no)\n"+Esc())
	res, err := c.readline.Readline()
	if err != nil {
		return false, "no", err
	}
	res = strings.TrimSpace(res)

	f := strings.Fields(res)
	if len(f) == 0 {
		return false, "no", nil
	}
	if res == "y" || res == "yes" || res == "ok" {
		return true, res, nil
	}
	return false, res, nil
}

func (c *Chat) Send(prompt string) (io.ReadCloser, error) {
	c.History = append(c.History, api.Message{Role: "user", Content: prompt})
	messages := []api.Message{{Role: "system", Content: c.SystemPrompt}}
	messages = append(messages, c.History...)
	payload := map[string]any{
		"model":    c.Model,
		"stream":   true,
		"messages": messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	rsp, err := c.client.Request("POST", "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

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
			data := &api.Data{}
			if err := json.Unmarshal([]byte(parts[1]), data); err != nil {
				return fmt.Errorf("failed to parse line %q: %s", line, err)
			}
			// TODO: nil checks
			if _, err := io.WriteString(w, data.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
		c.History = append(c.History, api.Message{
			Role:    "assistant",
			Content: reply.String(),
		})
		return nil
	}()
	return pr, nil
}

func (c *Chat) Run() error {
	for {
		prompt, err := c.GetPrompt()
		if err != nil {
			if err == io.EOF || err == readline.ErrInterrupt {
				return nil
			}
			return err
		}
		reply, err := c.Send(prompt)
		if err != nil {
			return err
		}
		if _, err := io.Copy(c.Display, reply); err != nil {
			return err
		}
		_ = reply.Close()
		if !c.Interactive {
			break
		}
	}
	return nil
}

func Esc(code ...int) string {
	if os.Getenv("NO_COLOR") != "" {
		return ""
	}
	codes := make([]string, len(code))
	for i := range code {
		codes[i] = strconv.Itoa(code[i])
	}
	return fmt.Sprintf("\x1b[%sm", strings.Join(codes, ";"))
}
