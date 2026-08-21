# 0026 — Opt-in global vault location per project

## Context

The v0.50 markdown vault (ADR 0024) always projects a project's Ledger into `<project-root>/.mom/vault/`. That is fine for most repositories, but a repository that already curates its own architectural documentation (an `adr/` or `docs/` directory checked into version control, as this repo does) ends up with two overlapping bodies of prose: the human-curated docs and the LLM-synthesized vault, both describing the same decisions. Users who dogfood _mom_ on such a repository want the vault stored somewhere it does not visually compete with the project's own docs, without giving up per-project vault content or having to duplicate it.

`.mom/` (including `.mom/vault/`) is already gitignored — the vault has never been a checked-in artifact. Project identity is already path-independent (ADR 0016): `.mom-project.yaml` declares a stable `id` at the project root, checked into version control, resolved the same way regardless of where the repository is cloned on any given machine. That existing mechanism is the natural key for relocating a vault without breaking cross-machine behaviour: the same `id` resolves to the same relative location under each machine's own `~/.mom`, even though the two machines' absolute paths differ.

A prior architecture (ADR 0006, superseded) escalated recall across a chain of filesystem-partitioned scopes (repo → org → user), and ADR 0009 explicitly dropped that model when consolidating on a single SQLite store, calling out that "filesystem-partitioned scopes... walked as a chain" was the wrong shape. This ADR does **not** resurrect that model. There is still exactly one vault per project, and no cross-project recall/merge is introduced — only the storage location of that single vault becomes configurable per project.

## Decision

### Opt-in `vault:` field in `.mom-project.yaml`

`.mom-project.yaml` gains an optional `vault` key:

```yaml
version: "1"
id: my-service
vault: global
```

- Absent, or `vault: project` — unchanged pre-existing behaviour: the vault lives at `<project-root>/.mom/vault/`.
- `vault: global` — the vault is stored under the central `~/.mom` store instead, keyed by the project's `id`: `~/.mom/vault/<id>/`.

The field is written by `mom project bind --vault project|global` (default `project`, so existing invocations and scripts are unaffected). Toggling the vault preference on an existing binding with the same `id` is not treated as an identity change — it does not require `--force`, unlike changing `id` itself. The `vault:` line is omitted from the file entirely when the value is `project`, so users who never opt in see byte-identical output to before this ADR.

### `~/.mom/vault/<project-id>/` as the global location

The global vault lives under `librarian.Dir()` (respecting the existing `MOM_VAULT` override used for local testing), keyed by the project's declared `id` rather than any path. Two clones of the same project on different machines — or two machines with different `$HOME` — each resolve to their own `~/.mom/vault/<id>/`, but always the *same* `<id>` subdirectory, because `id` is checked into the repository and does not depend on the absolute path. No symlink and no absolute path is ever written into a git-tracked file.

### Vault storage is decoupled from entry-file placement

The managed context block that points an agent at the vault must always live in the project's real `CLAUDE.md`/`AGENTS.md` — an agent working in a checked-out repository looks for these files at the project root regardless of where the vault content itself is stored. `services/projection.Writer` therefore separates two directories that used to be implicitly the same:

- `Root` — the project root; entry files are always written here.
- `VaultBase` — the directory vault content (files, `INDEX.md`, `.fold-state.json`) is read from and written to. Defaults to `VaultDir(Root)` (`<root>/.mom/vault`) when unset.

`RunOptions` (the shared entry point behind `mom vault fold`/`rebuild` and the daemon's auto-fold) gains matching `VaultBase` and `VaultRef` fields. `VaultRef` is the human-readable path rendered into the managed context block (`.mom/vault/` by default, or `~/.mom/vault/<id>/` for a global vault) so the agent is told the correct place to read from.

The per-project fold lock (`AcquireFoldLock`) is scoped to `VaultBase` directly rather than to the project root: it is placed as a dot-file (`.fold.lock`) inside the vault content directory itself. `pruneStaleConcepts` already exempts dot-files from a rebuild's prune pass, so this is safe for both the project-local and global cases and removes the need for a separate "root" concept purely for locking.

### CLI and daemon parity

`ingress/cli/vault.go` resolves the vault location once (`resolveVaultLocation`) from the project's binding and reuses it for `mom vault fold`, `mom vault rebuild`, and `mom vault status`. The watch daemon's auto-fold (`watch_autofold.go`) calls the same resolver, so a project that opted into the global vault gets it from both the manual CLI and the background daemon — the two callers never diverge, matching the existing invariant that governs the rest of the fold path.

### No new CLI surface for the vault content itself

There is no migration or listing command. Toggling `vault:` only changes where the *next* fold writes; it does not move existing files. A user switching modes deletes the stale location manually if they want a clean cutover — this mirrors how a manual `mom vault rebuild` is already the answer to "vault got into a state I want to reset," rather than adding new tooling.

## Rejected alternatives

- **Symlinking `<root>/.mom/vault` → `~/.mom/vault/<id>`.** Considered so the entry-file block could keep its existing relative reference unchanged. Rejected: it adds a filesystem object that must be created, verified, and repaired per machine, error-checked against a real directory existing on the other end, and re-created after any tooling that recreates `.mom/` from scratch. Pointing the entry file directly at the resolved path (as ADR 0016's own `id` already allows) is simpler and has no extra moving parts.
- **Keeping both a project-local and a global copy simultaneously.** Rejected — it reintroduces exactly the duplication this ADR exists to remove. Global mode replaces the project-local vault outright for that project.
- **A central `~/.mom/config.yaml` map of `project_id → location`.** Rejected in favour of the `.mom-project.yaml` field: the config file is machine-local and not checked into version control, so the choice would not travel with the repository across clones the way `.mom-project.yaml` already does for `id`. Declaring it beside `id` keeps one project-scoped source of truth instead of two.
- **A new `mom vault init` command.** Rejected — `mom project bind` is already the command that establishes a project's identity and is the natural place to also declare where its vault lives; adding a parallel command would duplicate that responsibility for no benefit.
- **Reviving scope-chain recall (ADR 0006).** Not proposed and explicitly out of scope: this ADR changes only where a single project's vault is stored, never how many vaults are consulted for a given project or how they are merged.

## Consequences

- A repository like this one can bind with `mom project bind --id mom --vault global` and get vault content out of its own working tree entirely, while the manual and daemon fold paths keep working identically otherwise.
- `services/projection.Writer` now has two independently meaningful directories (`Root`, `VaultBase`) instead of one; any future caller must be deliberate about which one governs a given file.
- `.mom-project.yaml`, still user-owned and checked into version control, is the single place declaring both a project's identity and its vault storage preference — consistent with ADR 0016's model of that file as project-scoped, portable metadata.
- Existing installs are unaffected: the `vault:` key is absent from every binding written before this ADR, and absent means the unchanged project-local behaviour.
