package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/bduffany/gpt-cli/internal/llm"
	"github.com/bduffany/gpt-cli/internal/openai"
	"github.com/chzyer/readline"
	"github.com/mattn/go-isatty"
)

type Chat struct {
	Model        string
	PromptReader io.Reader
	Interactive  bool
	Messages     []llm.Message

	Display io.Writer

	client   llm.CompletionClient
	readline *readline.Instance
	eof      bool
}

func New(client llm.CompletionClient, messages []llm.Message) (*Chat, error) {
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
		Messages:     slices.Clone(messages),
		Model:        openai.DefaultModel,
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
	c.Messages = append(c.Messages, llm.Message{
		Metadata: llm.MessageMetadata{
			Role: llm.RoleUser,
		},
		Payload: prompt,
	})
	completion, err := c.client.GetCompletion(ctx, c.Messages)
	if err != nil {
		return nil, err
	}
	return completion, nil
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
	var buf bytes.Buffer
	w := io.MultiWriter(c.Display, &buf)
	if _, err := io.Copy(w, reply); err != nil {
		return err
	}
	if len(buf.Bytes()) == 0 || buf.Bytes()[len(buf.Bytes())-1] != '\n' {
		// Ensure a newline so that the next prompt doesn't overwrite it.
		// TODO: this should probably be the responsibility of GetPrompt
		if _, err := io.WriteString(c.Display, "\n"); err != nil {
			return err
		}
	}
	c.Messages = append(c.Messages, llm.Message{
		Metadata: llm.MessageMetadata{
			Role: llm.RoleModel,
		},
		Payload: buf.String(),
	})
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
