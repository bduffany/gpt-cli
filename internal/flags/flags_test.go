package flags

import (
	"testing"
)

func TestBoolFlag(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode")

	if err := fs.Parse([]string{"--verbose"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*verbose {
		t.Error("expected verbose to be true")
	}
}

func TestBoolFlagDefault(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode")

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if *verbose {
		t.Error("expected verbose to be false")
	}
}

func TestStringFlag(t *testing.T) {
	fs := NewFlagSet()
	name := fs.String("name", "", "your name")

	if err := fs.Parse([]string{"--name", "alice"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if *name != "alice" {
		t.Errorf("expected name='alice', got %q", *name)
	}
}

func TestStringFlagEquals(t *testing.T) {
	fs := NewFlagSet()
	name := fs.String("name", "", "your name")

	if err := fs.Parse([]string{"--name=bob"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if *name != "bob" {
		t.Errorf("expected name='bob', got %q", *name)
	}
}

func TestStringFlagDefault(t *testing.T) {
	fs := NewFlagSet()
	name := fs.String("name", "default", "your name")

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if *name != "default" {
		t.Errorf("expected name='default', got %q", *name)
	}
}

func TestShortFlag(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode", Short("v"))

	if err := fs.Parse([]string{"-v"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*verbose {
		t.Error("expected verbose to be true")
	}
}

func TestShortStringFlag(t *testing.T) {
	fs := NewFlagSet()
	output := fs.String("output", "", "output file", Short("o"))

	if err := fs.Parse([]string{"-o", "file.txt"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if *output != "file.txt" {
		t.Errorf("expected output='file.txt', got %q", *output)
	}
}

func TestShortStringFlagEquals(t *testing.T) {
	fs := NewFlagSet()
	output := fs.String("output", "", "output file", Short("o"))

	if err := fs.Parse([]string{"-o=file.txt"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if *output != "file.txt" {
		t.Errorf("expected output='file.txt', got %q", *output)
	}
}

func TestCombinedShortFlags(t *testing.T) {
	fs := NewFlagSet()
	a := fs.Bool("aaa", false, "flag a", Short("a"))
	b := fs.Bool("bbb", false, "flag b", Short("b"))
	c := fs.Bool("ccc", false, "flag c", Short("c"))

	if err := fs.Parse([]string{"-abc"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*a {
		t.Error("expected a to be true")
	}
	if !*b {
		t.Error("expected b to be true")
	}
	if !*c {
		t.Error("expected c to be true")
	}
}

func TestCombinedShortFlagsPartial(t *testing.T) {
	fs := NewFlagSet()
	a := fs.Bool("aaa", false, "flag a", Short("a"))
	b := fs.Bool("bbb", false, "flag b", Short("b"))
	_ = fs.Bool("ccc", false, "flag c", Short("c"))

	if err := fs.Parse([]string{"-ab"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*a {
		t.Error("expected a to be true")
	}
	if !*b {
		t.Error("expected b to be true")
	}
}

func TestHelpFlag(t *testing.T) {
	fs := NewFlagSet()
	_ = fs.Bool("verbose", false, "enable verbose mode")

	err := fs.Parse([]string{"--help"})
	if err != ErrHelp {
		t.Errorf("expected ErrHelp, got %v", err)
	}
}

func TestHelpFlagShort(t *testing.T) {
	fs := NewFlagSet()
	_ = fs.Bool("verbose", false, "enable verbose mode")

	err := fs.Parse([]string{"-h"})
	if err != ErrHelp {
		t.Errorf("expected ErrHelp, got %v", err)
	}
}

func TestHelpInCombinedFlags(t *testing.T) {
	fs := NewFlagSet()
	_ = fs.Bool("aaa", false, "flag a", Short("a"))

	err := fs.Parse([]string{"-ah"})
	if err != ErrHelp {
		t.Errorf("expected ErrHelp, got %v", err)
	}
}

func TestUnknownFlag(t *testing.T) {
	fs := NewFlagSet()

	err := fs.Parse([]string{"--unknown"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestStringFlagMissingValue(t *testing.T) {
	fs := NewFlagSet()
	_ = fs.String("name", "", "your name")

	err := fs.Parse([]string{"--name"})
	if err == nil {
		t.Error("expected error for missing value")
	}
}

func TestDoubleDashSeparator(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode")

	if err := fs.Parse([]string{"--verbose", "--", "--not-a-flag", "arg"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*verbose {
		t.Error("expected verbose to be true")
	}
	args := fs.Args()
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "--not-a-flag" {
		t.Errorf("expected args[0]='--not-a-flag', got %q", args[0])
	}
	if args[1] != "arg" {
		t.Errorf("expected args[1]='arg', got %q", args[1])
	}
}

func TestNonFlagArgs(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode")

	if err := fs.Parse([]string{"--verbose", "arg1", "arg2"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*verbose {
		t.Error("expected verbose to be true")
	}
	args := fs.Args()
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "arg1" {
		t.Errorf("expected args[0]='arg1', got %q", args[0])
	}
}

func TestMixedFlags(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode", Short("v"))
	output := fs.String("output", "", "output file", Short("o"))
	force := fs.Bool("force", false, "force overwrite", Short("f"))

	if err := fs.Parse([]string{"-v", "--output", "out.txt", "-f", "input.txt"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*verbose {
		t.Error("expected verbose to be true")
	}
	if *output != "out.txt" {
		t.Errorf("expected output='out.txt', got %q", *output)
	}
	if !*force {
		t.Error("expected force to be true")
	}
	args := fs.Args()
	if len(args) != 1 || args[0] != "input.txt" {
		t.Errorf("expected args=['input.txt'], got %v", args)
	}
}

func TestCombinedFlagsNotAllValid(t *testing.T) {
	fs := NewFlagSet()
	_ = fs.Bool("aaa", false, "flag a", Short("a"))
	// "x" is not defined, so -ax should fail

	err := fs.Parse([]string{"-ax"})
	if err == nil {
		t.Error("expected error for invalid combined flag")
	}
}

func TestSingleDashFlag(t *testing.T) {
	fs := NewFlagSet()
	v := fs.Bool("v", false, "verbose")

	if err := fs.Parse([]string{"-v"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*v {
		t.Error("expected v to be true")
	}
}

func TestLongFlagWithSingleDash(t *testing.T) {
	fs := NewFlagSet()
	verbose := fs.Bool("verbose", false, "enable verbose mode")

	// -verbose should work (single dash with long name)
	if err := fs.Parse([]string{"-verbose"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !*verbose {
		t.Error("expected verbose to be true")
	}
}
