package auto

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	_ "embed"

	"github.com/bduffany/gpt-cli/internal/chat"
	"github.com/bduffany/gpt-cli/internal/log"
	"github.com/chzyer/readline"
)

const aiPS1 = "\x1b[90mgpt>\x1b[m "

var availableCommands = []CommandSpec{
	{
		Cmd:  "prompt",
		Desc: "Requests the user for the next prompt and returns the result.",
		Run:  runPrompt,
	},
	{
		Cmd:  "cat",
		Args: "FILES ...",
		Desc: "Returns the concatenated contents of one or more files.",
		Run:  safeShellCommand("cat"),
	},
	{
		Cmd:  "ls",
		Args: "PATH ...",
		Desc: "Runs ls -la on the given paths and returns the result.",
		Run:  safeShellCommand("ls", "-la"),
	},
	{
		Cmd:  "write",
		Args: "PATH",
		Desc: "Writes a file with permissions 0644. For this command only, you are allowed to provide additional output on the lines following the command. Any additional lines are written to the file.",
		Run:  runWrite,
	},
	{
		Cmd:  "curl",
		Args: "URL",
		Desc: "Issue an HTTP GET request. You can use this for things like searching google or requesting from https://api.github.com. The first line will contain the response code. Next a blank line. Following that, the HTTP response body.",
		Run:  runHTTPGet,
	},
}

//go:embed auto.md
var promptTemplate string

type FixableError struct {
	Err  error
	Hint string
}

func (e *FixableError) Error() string {
	return fmt.Sprintf("%s\n# GPT: %s", e.Err, e.Hint)
}

func Run(ctx context.Context, c *chat.Chat) error {
	c.SystemPrompt = systemPrompt()
	input := ""
	log.Debugf("Beginning session.")
	for {
		err := (func() error {
			h := &ReplyHandler{chat: c}
			r, err := c.Send(ctx, input)
			if err != nil {
				return err
			}
			defer r.Close()

			output, err := h.Handle(r)
			if e, ok := err.(*FixableError); ok {
				input = e.Error()
				return nil
			}

			// Next input is based on the output of the command.
			input = output
			return err
		})()
		if err == io.EOF || err == readline.ErrInterrupt {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

type ReplyHandler struct {
	chat       *chat.Chat
	comment    string
	parsedArgs bool
	args       []string
	buf        bytes.Buffer
	pw         *io.PipeWriter
	cmd        *Command
	result     chan Result
}

func (h *ReplyHandler) Handle(r io.Reader) (string, error) {
	io.WriteString(h.chat.Display, aiPS1)

	_, err := io.Copy(h, r)
	if err != nil {
		return "", err
	}
	if err := h.consume(true /*=finalize*/); err != nil {
		return "", err
	}
	if h.cmd != nil {
		io.WriteString(h.chat.Display, "\n")
		res := <-h.result
		return res.Val, res.Err
	}
	return "", nil
}

func (h *ReplyHandler) Write(p []byte) (n int, err error) {
	h.buf.Write(p)
	err = h.consume(false /*=finalize*/)
	return len(p), err
}

func (h *ReplyHandler) consume(finalize bool) error {
	// Parse comment and arguments.
	for h.buf.Len() > 0 && !h.parsedArgs {
		b := h.buf.Bytes()

		if h.comment == "" && b[0] != '#' {
			return &FixableError{
				Err:  fmt.Errorf("unexpected input %q", string(b)),
				Hint: "Each command must be preceded by a comment line starting with '#' that explains the command.",
			}
		}

		// If we haven't yet read the full comment, don't try to tokenize it.
		// Just consume until newline.
		if !strings.HasSuffix(h.comment, "\n") {
			part := b
			newline := false
			if idx := bytes.Index(b, []byte{'\n'}); idx >= 0 {
				part = b[:idx+1]
				newline = true
			}
			h.chat.Display.Write(part)
			if newline {
				io.WriteString(h.chat.Display, aiPS1)
			}
			h.comment += string(part)
			h.buf.Next(len(part))
			continue
		}

		// Now we're parsing the command and args. Look for either a space (end of
		// arg) or a newline (end of command).
		idx := -1
		for i := range b {
			if b[i] == '\n' || b[i] == ' ' {
				h.parsedArgs = b[i] == '\n'
				idx = i
				break
			}
		}
		if idx < 0 {
			if finalize {
				idx = len(b)
			} else {
				break
			}
		}
		h.args = append(h.args, string(b[:idx]))
		h.chat.Display.Write(h.buf.Next(idx + 1))
	}

	// If we've parsed the command and args, start the command.
	if h.cmd == nil && h.parsedArgs {
		for _, spec := range availableCommands {
			if spec.Cmd != h.args[0] {
				continue
			}
			h.result = make(chan Result, 1)
			pr, pw := io.Pipe()
			h.cmd = &Command{
				Spec:  &spec,
				Chat:  h.chat,
				args:  h.args[1:],
				input: pr,
			}
			h.pw = pw
			h.args = h.args[1:]
			go func() {
				output, err := h.cmd.Spec.Run(h.cmd)
				pr.Close()
				h.result <- Result{output, err}
			}()
			break
		}
		if h.cmd == nil {
			return &FixableError{
				Err:  fmt.Errorf("invalid command %q", h.args[0]),
				Hint: "You can only issue commands from the available commands list. If you are stuck, use the prompt command to ask for directions.",
			}
		}
	}

	// After we've completely parsed args, all subsequent output gets piped to the
	// command. If we completely parsed arguments and failed to find the command,
	// that's an error.
	if h.parsedArgs {
		if h.cmd == nil {
			return &FixableError{
				Err:  fmt.Errorf("failed to parse command"),
				Hint: "Your reply must contain a comment starting with '#', then a command.",
			}
		}
		h.pw.Write(h.buf.Next(h.buf.Len()))
		if finalize {
			h.pw.Close()
		}
	}

	return nil
}

type CommandSpec struct {
	Cmd  string
	Args string
	Desc string
	Run  func(*Command) (string, error)
}

func (c *CommandSpec) String() string {
	return fmt.Sprintf("%s %s", c.Cmd, c.Args)
}

func systemPrompt() string {
	specs := ""
	for _, c := range availableCommands {
		specs += "- command: " + c.Cmd + "\n"
		specs += "  description: " + c.Desc + "\n"
	}
	return strings.Replace(promptTemplate, "#{COMMANDS}", specs, 1)
}

type Command struct {
	Spec *CommandSpec
	Chat *chat.Chat

	args   []string // does not include command name
	input  *io.PipeReader
	result chan Result
}

type Result struct {
	Val string
	Err error
}

func GetPrompt() (string, error) {
	return "", nil
}

func runPrompt(cmd *Command) (string, error) {
	return cmd.Chat.GetPrompt()
}

func safeShellCommand(command string, flags ...string) func(cmd *Command) (string, error) {
	return func(cmd *Command) (string, error) {
		c := exec.Command(command, append(flags, cmd.args...)...)
		b, err := c.CombinedOutput()
		if err != nil {
			return "", &FixableError{
				Err:  fmt.Errorf("%s", string(b)),
				Hint: "The command failed. Try something else, or prompt on how to proceed",
			}
		}
		return string(b), nil
	}
}

func runWrite(cmd *Command) (string, error) {
	if len(cmd.args) > 1 {
		return "", &FixableError{
			Err:  fmt.Errorf("unexpected arg %q", cmd.args[1]),
			Hint: "The write command only accepts one filename arg. If you are trying to write this as output to the file, note that output must come on the line after the command.",
		}
	}
	b, err := io.ReadAll(io.TeeReader(cmd.input, cmd.Chat.Display))
	if err != nil {
		return "", err
	}
	path := cmd.args[0]
	log.Debugf("Read all input from gpt. Confirming.")
	ok, reply, err := cmd.Chat.Confirmf("Write the above contents to %q?", path)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", &FixableError{
			Err:  fmt.Errorf("permission denied"),
			Hint: fmt.Sprintf("I denied your request: %q", reply),
		}
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", &FixableError{
			Err:  err,
			Hint: "The file failed to write.",
		}
	}
	return "", nil
}

func runHTTPGet(cmd *Command) (string, error) {
	if len(cmd.args) != 1 {
		return "", &FixableError{
			Err:  fmt.Errorf("expected exactly one URL arg"),
			Hint: "Example curl command: curl https://google.com/search?q=Hello",
		}
	}
	res, err := http.Get(cmd.args[0])
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	reply := res.Status + "\n\n"
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", &FixableError{
			Err:  fmt.Errorf("failed to read response body: %w", err),
			Hint: "Does this seem like a transient error? Maybe retry it?",
		}
	}
	return reply + string(b), nil
}
