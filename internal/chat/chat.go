package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/bduffany/gpt-cli/internal/api"
	"github.com/chzyer/readline"
	"github.com/mattn/go-isatty"
)

type Chat struct {
	Model        string
	PromptReader io.Reader
	Interactive  bool
	Messages     []api.Message

	Display io.Writer

	client   *api.Client
	readline *readline.Instance
	eof      bool
}

func New(client *api.Client, messages []api.Message) (*Chat, error) {
	var rl *readline.Instance
	interactive := isatty.IsTerminal(os.Stdin.Fd())
	var pr io.Reader
	if !interactive {
		pr = os.Stdin
	}
	return &Chat{
		client:       client,
		readline:     rl,
		Display:      os.Stdout,
		Messages:     append([]api.Message{}, messages...),
		Model:        api.DefaultModel,
		Interactive:  interactive,
		PromptReader: pr,
	}, nil
}

func (c *Chat) GetPrompt() (string, error) {
	if c.eof {
		return "", io.EOF
	}

	if c.PromptReader != nil {
		b, err := io.ReadAll(c.PromptReader)
		if !c.Interactive {
			c.eof = true
		}
		return string(b), err
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

func (c *Chat) Infof(format string, args ...any) {
	io.WriteString(c.Display, Esc(92)+fmt.Sprintf(format, args...)+"\n"+Esc())
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

func (c *Chat) Send(ctx context.Context, prompt string) (io.ReadCloser, error) {
	c.Messages = append(c.Messages, api.Message{Role: "user", Content: prompt})
	payload := map[string]any{
		"model":    c.Model,
		"stream":   true,
		"messages": c.Messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	rsp, err := c.client.Request(ctx, "POST", "/v1/chat/completions", bytes.NewReader(body))
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
		if scanner.Err() != nil {
			return scanner.Err()
		}
		c.Messages = append(c.Messages, api.Message{
			Role:    "assistant",
			Content: reply.String(),
		})
		return nil
	}()
	return pr, nil
}

// Run starts the prompting loop for the chat, reading from the prompt source
// until inputs are exhausted.
func (c *Chat) Run(ctx context.Context) error {
	for {
		if err := c.readAndExecutePrompt(ctx); err != nil {
			if err == io.EOF || err == readline.ErrInterrupt {
				return nil
			}
			return err
		}
		if !c.Interactive {
			break
		}
	}
	return nil
}

func (c *Chat) readAndExecutePrompt(ctx context.Context) (err error) {
	prompt, err := c.GetPrompt()
	if err != nil {
		return err
	}
	// When pressing Ctrl+C during a reply, stop the current request but don't
	// return an error during program execution. This allows long replies to be
	// interrupted without terminating the session completely.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT)
	defer stop()
	defer func() {
		if errors.Is(err, context.Canceled) {
			// Context was canceled due to Ctrl+C; treat this as a non-error.
			err = nil
			// Print a blank line since otherwise the readline lib overwrites any
			// partial output on the last line.
			io.WriteString(c.Display, "\n")
		}
	}()

	reply, err := c.Send(ctx, prompt)
	if err != nil {
		return err
	}
	defer reply.Close()
	if _, err := io.Copy(c.Display, reply); err != nil {
		return err
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
