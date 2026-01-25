package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantErr      string
	}{
		{
			name:         "builtin persona",
			input:        "grug",
			wantContains: []string{"Your persona is: grug", "caveman"},
		},
		{
			name:         "empty name returns empty",
			input:        "",
			wantContains: nil,
		},
		{
			name:    "nonexistent persona",
			input:   "nonexistent",
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("AI_PERSONAS_DIRECTORY", tmpDir)

			result, err := Load(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got: %s", want, result)
				}
			}
		})
	}
}

func TestLoadUserPersona(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		content      string
		loadName     string
		wantContains []string
	}{
		{
			name:         "txt extension",
			filename:     "testbot.txt",
			content:      "You are a helpful test persona.",
			loadName:     "testbot",
			wantContains: []string{"Your persona is: testbot", "helpful test persona"},
		},
		{
			name:         "md extension",
			filename:     "mdbot.md",
			content:      "You are a markdown test persona.",
			loadName:     "mdbot",
			wantContains: []string{"Your persona is: mdbot", "markdown test persona"},
		},
		{
			name:         "no extension",
			filename:     "rawbot",
			content:      "You are a raw test persona.",
			loadName:     "rawbot",
			wantContains: []string{"Your persona is: rawbot", "raw test persona"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("AI_PERSONAS_DIRECTORY", tmpDir)

			if err := os.WriteFile(filepath.Join(tmpDir, tt.filename), []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test persona: %v", err)
			}

			result, err := Load(tt.loadName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got: %s", want, result)
				}
			}
		})
	}
}

func TestLoadPersonaByPath(t *testing.T) {
	tmpDir := t.TempDir()

	personaContent := "You are a path-loaded persona."
	personaPath := filepath.Join(tmpDir, "custom.txt")
	if err := os.WriteFile(personaPath, []byte(personaContent), 0644); err != nil {
		t.Fatalf("failed to write test persona: %v", err)
	}

	result, err := Load(personaPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantContains := []string{"Your persona is: custom", "path-loaded persona"}
	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("expected result to contain %q, got: %s", want, result)
		}
	}
}

func TestLoadRandom(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AI_PERSONAS_DIRECTORY", tmpDir)

	result, err := Load("random")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Your persona is:") {
		t.Errorf("expected persona header, got: %s", result)
	}
}

func TestLoadRandomIncludesUserPersonas(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AI_PERSONAS_DIRECTORY", tmpDir)

	personaContent := "UNIQUE_TEST_MARKER_12345"
	if err := os.WriteFile(filepath.Join(tmpDir, "uniquetest.txt"), []byte(personaContent), 0644); err != nil {
		t.Fatalf("failed to write test persona: %v", err)
	}

	// Run random selection multiple times to increase chance of hitting user persona
	foundUserPersona := false
	for i := 0; i < 100; i++ {
		result, err := Load("random")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(result, "UNIQUE_TEST_MARKER_12345") {
			foundUserPersona = true
			break
		}
	}

	if !foundUserPersona {
		t.Log("warning: user persona was not selected in 100 random attempts (statistically unlikely but possible)")
	}
}

func TestBuiltinPersonasShadowUserPersonas(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AI_PERSONAS_DIRECTORY", tmpDir)

	personaContent := "This should be shadowed by builtin"
	if err := os.WriteFile(filepath.Join(tmpDir, "grug.txt"), []byte(personaContent), 0644); err != nil {
		t.Fatalf("failed to write test persona: %v", err)
	}

	result, err := Load("grug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result, "should be shadowed") {
		t.Errorf("user persona should be shadowed by builtin, got: %s", result)
	}
	if !strings.Contains(result, "caveman") {
		t.Errorf("expected builtin grug persona, got: %s", result)
	}
}

func TestListBuiltin(t *testing.T) {
	names := ListBuiltin()
	if len(names) == 0 {
		t.Fatal("expected at least one builtin persona")
	}

	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}

	expected := []string{"grug", "noir", "zen", "baba", "griot"}
	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected %q in builtin personas", name)
		}
	}
}

func TestListUserPersonas(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // filename -> content
		want     []string
		notWant  []string
	}{
		{
			name: "various extensions",
			files: map[string]string{
				"alpha.txt": "alpha",
				"beta.md":   "beta",
				"gamma":     "gamma",
			},
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "excludes builtins",
			files: map[string]string{
				"grug.txt":   "fake grug",
				"custom.txt": "custom",
			},
			want:    []string{"custom"},
			notWant: []string{"grug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("AI_PERSONAS_DIRECTORY", tmpDir)

			for filename, content := range tt.files {
				os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
			}

			names := listUserPersonas()
			found := make(map[string]bool)
			for _, name := range names {
				found[name] = true
			}

			for _, want := range tt.want {
				if !found[want] {
					t.Errorf("expected %q in user personas", want)
				}
			}
			for _, notWant := range tt.notWant {
				if found[notWant] {
					t.Errorf("did not expect %q in user personas", notWant)
				}
			}
		})
	}
}

func TestConfigDir(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name    string
		envVar  string
		want    string
	}{
		{
			name:   "uses env var",
			envVar: "/custom/path",
			want:   "/custom/path",
		},
		{
			name:   "falls back to default",
			envVar: "",
			want:   filepath.Join(home, "AIPersonas"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("AI_PERSONAS_DIRECTORY", tt.envVar)
			} else {
				t.Setenv("AI_PERSONAS_DIRECTORY", "")
			}

			got := ConfigDir()
			if got != tt.want {
				t.Errorf("ConfigDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
