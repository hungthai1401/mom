package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlobalContextPath_MatchesWhereBlockIsWritten pins the invariant that
// broke `mom doctor`: the path an adapter advertises must be the exact path
// its own GenerateGlobalContextFile writes to. Doctor and uninstall both read
// GlobalContextPath, so any adapter that reports one location and writes to
// another silently reports a healthy install as broken.
func TestGlobalContextPath_MatchesWhereBlockIsWritten(t *testing.T) {
	for _, name := range []string{"claude", "codex", "pi", "droid"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("CODEX_HOME", "")

			adapter, ok := NewRegistry(t.TempDir()).Get(name)
			if !ok {
				t.Fatalf("%s not registered", name)
			}
			global, ok := adapter.(GlobalAdapter)
			if !ok {
				t.Fatalf("%s does not implement GlobalAdapter", name)
			}

			declared, err := global.GlobalContextPath()
			if err != nil {
				t.Fatal(err)
			}
			if err := global.GenerateGlobalContextFile(Config{}, nil, nil, nil); err != nil {
				t.Fatal(err)
			}

			data, err := os.ReadFile(declared)
			if err != nil {
				t.Fatalf("nothing written at declared path %s: %v", declared, err)
			}
			if !strings.Contains(string(data), momBlockStart) {
				t.Errorf("%s carries no MOM block", declared)
			}
		})
	}
}

// TestGlobalContextPaths_CoversEveryGlobalAdapter guards the other half of the
// old bug: uninstall kept its own list of home files and omitted pi, so the
// MOM block survived an uninstall.
func TestGlobalContextPaths_CoversEveryGlobalAdapter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	r := NewRegistry(t.TempDir())
	paths := r.GlobalContextPaths()
	for _, adapter := range r.All() {
		if _, ok := adapter.(GlobalAdapter); !ok {
			continue
		}
		if paths[adapter.Name()] == "" {
			t.Errorf("GlobalContextPaths is missing %s", adapter.Name())
		}
	}
}

// TestGlobalContextPaths_HonorsCodexHome proves the paths track adapter
// behaviour rather than a hardcoded ~/.codex, which the old doctor list did
// not.
func TestGlobalContextPaths_HonorsCodexHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	got := NewRegistry(t.TempDir()).GlobalContextPaths()["codex"]
	if want := filepath.Join(codexHome, "AGENTS.md"); got != want {
		t.Errorf("codex path = %q, want %q", got, want)
	}
}
