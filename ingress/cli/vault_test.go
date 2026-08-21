package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/momhq/mom/services/projection"
	"github.com/momhq/mom/shared/project"
)

// ADR 0026: resolveVaultLocation defaults to the pre-existing
// project-local vault (<root>/.mom/vault, ".mom/vault/") unless root's
// binding declares `vault: global`, in which case it resolves to a
// project-id-keyed directory under the central ~/.mom store (respecting
// the MOM_VAULT test override, same as the rest of the daemon/CLI).
func TestResolveVaultLocation(t *testing.T) {
	t.Run("project-local when no binding", func(t *testing.T) {
		root := t.TempDir()
		base, ref, err := resolveVaultLocation(root, "some-id")
		if err != nil {
			t.Fatalf("resolveVaultLocation: %v", err)
		}
		if want := projection.VaultDir(root); base != want {
			t.Errorf("base = %q, want %q", base, want)
		}
		if ref != ".mom/vault/" {
			t.Errorf("ref = %q, want %q", ref, ".mom/vault/")
		}
	})

	t.Run("project-local when binding declares vault: project", func(t *testing.T) {
		root := t.TempDir()
		if err := project.WriteBinding(root, "some-id", false, false); err != nil {
			t.Fatalf("WriteBinding: %v", err)
		}
		base, ref, err := resolveVaultLocation(root, "some-id")
		if err != nil {
			t.Fatalf("resolveVaultLocation: %v", err)
		}
		if want := projection.VaultDir(root); base != want {
			t.Errorf("base = %q, want %q", base, want)
		}
		if ref != ".mom/vault/" {
			t.Errorf("ref = %q, want %q", ref, ".mom/vault/")
		}
	})

	t.Run("global when binding declares vault: global", func(t *testing.T) {
		isolated := t.TempDir()
		t.Setenv("MOM_VAULT", filepath.Join(isolated, ".mom", "mom.db"))

		root := t.TempDir()
		if err := project.WriteBinding(root, "some-id", false, true); err != nil {
			t.Fatalf("WriteBinding: %v", err)
		}
		base, ref, err := resolveVaultLocation(root, "some-id")
		if err != nil {
			t.Fatalf("resolveVaultLocation: %v", err)
		}
		wantBase := filepath.Join(isolated, ".mom", "vault", "some-id")
		if base != wantBase {
			t.Errorf("base = %q, want %q", base, wantBase)
		}
		if ref != "~/.mom/vault/some-id/" {
			t.Errorf("ref = %q, want %q", ref, "~/.mom/vault/some-id/")
		}
	})

	t.Run("global base is independent of root", func(t *testing.T) {
		isolated := t.TempDir()
		t.Setenv("MOM_VAULT", filepath.Join(isolated, ".mom", "mom.db"))

		rootA := t.TempDir()
		rootB := t.TempDir()
		if err := project.WriteBinding(rootA, "shared-id", false, true); err != nil {
			t.Fatalf("WriteBinding rootA: %v", err)
		}
		if err := project.WriteBinding(rootB, "shared-id", false, true); err != nil {
			t.Fatalf("WriteBinding rootB: %v", err)
		}
		baseA, _, err := resolveVaultLocation(rootA, "shared-id")
		if err != nil {
			t.Fatalf("resolveVaultLocation rootA: %v", err)
		}
		baseB, _, err := resolveVaultLocation(rootB, "shared-id")
		if err != nil {
			t.Fatalf("resolveVaultLocation rootB: %v", err)
		}
		if baseA != baseB {
			t.Errorf("global vault base should key off project id, not root: %q != %q", baseA, baseB)
		}
	})
}

// Sanity: the resolved global base actually exists under a real .mom
// directory shape once something writes to it (mirrors what
// projection.VaultDir/AcquireFoldLock expect).
func TestResolveVaultLocation_GlobalBaseIsWritable(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("MOM_VAULT", filepath.Join(isolated, ".mom", "mom.db"))

	root := t.TempDir()
	if err := project.WriteBinding(root, "writable-id", false, true); err != nil {
		t.Fatalf("WriteBinding: %v", err)
	}
	base, _, err := resolveVaultLocation(root, "writable-id")
	if err != nil {
		t.Fatalf("resolveVaultLocation: %v", err)
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "INDEX.md"), []byte("# demo\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
