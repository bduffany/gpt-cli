// Package flags provides custom flag parsing with support for short/long flag aliases.
package flags

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrHelp is returned when -h or --help is invoked.
var ErrHelp = errors.New("help requested")

// Option is a functional option for configuring flags.
type Option func(*flagConfig)

type flagConfig struct {
	short string
}

// Short specifies a short flag name (single character, used with single dash).
func Short(name string) Option {
	return func(c *flagConfig) {
		c.short = name
	}
}

// FlagSet is a custom flag set that supports short/long aliases.
type FlagSet struct {
	boolFlags   map[string]*boolFlag
	stringFlags map[string]*stringFlag
	flags       []any // ordered list for help output
	args        []string
	parsed      bool
}

type boolFlag struct {
	value *bool
	name  string
	short string
	usage string
}

type stringFlag struct {
	value    *string
	name     string
	short    string
	defValue string
	usage    string
}

// NewFlagSet creates a new FlagSet.
func NewFlagSet() *FlagSet {
	return &FlagSet{
		boolFlags:   make(map[string]*boolFlag),
		stringFlags: make(map[string]*stringFlag),
	}
}

// Bool defines a bool flag with the given name, default value, and usage string.
// Use Short("x") option to add a single-character alias.
func (f *FlagSet) Bool(name string, defValue bool, usage string, opts ...Option) *bool {
	cfg := &flagConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	p := new(bool)
	*p = defValue
	bf := &boolFlag{value: p, name: name, short: cfg.short, usage: usage}

	f.boolFlags[name] = bf
	if cfg.short != "" {
		f.boolFlags[cfg.short] = bf
	}
	f.flags = append(f.flags, bf)
	return p
}

// String defines a string flag with the given name, default value, and usage string.
// Use Short("x") option to add a single-character alias.
func (f *FlagSet) String(name, defValue, usage string, opts ...Option) *string {
	cfg := &flagConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	p := new(string)
	*p = defValue
	sf := &stringFlag{value: p, name: name, short: cfg.short, defValue: defValue, usage: usage}

	f.stringFlags[name] = sf
	if cfg.short != "" {
		f.stringFlags[cfg.short] = sf
	}
	f.flags = append(f.flags, sf)
	return p
}

// Args returns the non-flag arguments after parsing.
func (f *FlagSet) Args() []string {
	return f.args
}

// Parse parses flag definitions from the argument list.
// It stops parsing at the first non-flag argument or after "--".
func (f *FlagSet) Parse(arguments []string) error {
	f.parsed = true
	f.args = nil

	i := 0
	for i < len(arguments) {
		arg := arguments[i]

		// Stop at "--"
		if arg == "--" {
			f.args = append(f.args, arguments[i+1:]...)
			return nil
		}

		// Non-flag argument
		if !strings.HasPrefix(arg, "-") {
			f.args = append(f.args, arguments[i:]...)
			return nil
		}

		// Handle combined short flags like -gt (expands to -g -t)
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 && !strings.Contains(arg, "=") {
			// Check if all characters are valid short bool flags
			chars := arg[1:]
			allValid := true
			for _, c := range chars {
				name := string(c)
				if name == "h" {
					f.PrintUsage()
					return ErrHelp
				}
				if _, ok := f.boolFlags[name]; !ok {
					allValid = false
					break
				}
			}
			if allValid {
				for _, c := range chars {
					bf := f.boolFlags[string(c)]
					*bf.value = true
				}
				i++
				continue
			}
		}

		// Parse flag
		name := arg
		value := ""
		hasValue := false

		// Handle --flag=value or -f=value
		if idx := strings.Index(arg, "="); idx != -1 {
			name = arg[:idx]
			value = arg[idx+1:]
			hasValue = true
		}

		// Strip leading dashes for lookup
		lookupName := strings.TrimLeft(name, "-")

		// Handle -h/--help
		if lookupName == "h" || lookupName == "help" {
			f.PrintUsage()
			return ErrHelp
		}

		// Check bool flags
		if bf, ok := f.boolFlags[lookupName]; ok {
			*bf.value = true
			i++
			continue
		}

		// Check string flags
		if sf, ok := f.stringFlags[lookupName]; ok {
			if !hasValue {
				if i+1 >= len(arguments) {
					return fmt.Errorf("flag %s requires a value", name)
				}
				i++
				value = arguments[i]
			}
			*sf.value = value
			i++
			continue
		}

		return fmt.Errorf("unknown flag: %s", name)
	}

	return nil
}

// PrintUsage prints usage information for all defined flags to stderr.
func (f *FlagSet) PrintUsage() {
	fmt.Fprintln(os.Stderr, "Flags:")

	for _, fl := range f.flags {
		switch v := fl.(type) {
		case *boolFlag:
			printFlagUsage(v.short, v.name, "", v.usage)
		case *stringFlag:
			printFlagUsage(v.short, v.name, v.defValue, v.usage)
		}
	}
}

func printFlagUsage(short, long, defValue, usage string) {
	var names []string
	if short != "" {
		names = append(names, "-"+short)
	}
	names = append(names, "--"+long)
	nameStr := strings.Join(names, ", ")
	if defValue != "" {
		fmt.Fprintf(os.Stderr, "  %s\n    \t%s (default: %s)\n", nameStr, usage, defValue)
	} else {
		fmt.Fprintf(os.Stderr, "  %s\n    \t%s\n", nameStr, usage)
	}
}
