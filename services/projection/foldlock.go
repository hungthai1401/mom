package projection

import (
	"errors"
	"path/filepath"

	"github.com/momhq/mom/shared/flock"
)

// ErrFoldLocked reports that another fold (a concurrent `mom vault fold`
// or the daemon's auto-fold) already holds the project's fold lock.
// Callers should surface it and try again later, never bypass it.
var ErrFoldLocked = errors.New("another fold is already running for this project")

// FoldLock is an exclusive, per-project, cross-process fold lock. It guards
// the vault write path (fold state, vault files, entry-file block) so a
// daemon auto-fold and a manual `mom vault fold` can never interleave on the
// same project. Cross-platform via shared/flock.
type FoldLock struct {
	l *flock.Handle
}

// foldLockPath returns <vaultBase>/.fold.lock. It lives as a dot-file
// directly inside the vault content directory: pruneStaleConcepts never
// deletes dot-files, so this is safe for both the project-local vault
// base (<root>/.mom/vault) and the opt-in global vault base
// (~/.mom/vault/<project-id>, which has no natural project root to keep
// the lock "beside").
func foldLockPath(vaultBase string) string {
	return filepath.Join(vaultBase, ".fold.lock")
}

// AcquireFoldLock takes the project fold lock (scoped by vault base
// directory) without blocking. Returns ErrFoldLocked when another
// process holds it. Callers must Release.
func AcquireFoldLock(vaultBase string) (*FoldLock, error) {
	l, err := flock.TryLock(foldLockPath(vaultBase))
	if err != nil {
		if errors.Is(err, flock.ErrLocked) {
			return nil, ErrFoldLocked
		}
		return nil, err
	}
	return &FoldLock{l: l}, nil
}

// Release drops the lock. Safe to call once.
func (l *FoldLock) Release() {
	if l == nil {
		return
	}
	l.l.Unlock()
	l.l = nil
}
