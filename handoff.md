# RACK WARS session handoff

Written 2026-11-05 against `main` at `821ea52`, with uncommitted Phase 4 work.
Active expert for this implementation: `expert-engineer-peirce`.
User requested that the identified failures and integration issues be resolved,
then this handoff updated for their local LLM, and subsequently requested a commit
of all work. This handoff is included in that commit despite its ignore rule.
The verification below describes the tree prepared for that commit; use git log -1
to identify the resulting commit.

## Start here

1. Read `git status --short` and include untracked files in review. Preserve the existing Phase 4 work; it is not disposable scratch. `handoff.md` is intentionally gitignored, so ordinary status/diff will not show this file.
2. Read Phase 3 evidence in `TUI-TASKS.md` and Phase 4 requirements in `TUI-PLAN.md`. The required platform gates remain open; do not mark them passed from the automated tests below.
3. Recheck with PowerShell: `$env:GOCACHE=Join-Path $env:TEMP 'rackwars-go-cache'; go test ./... -count=1`. The default user Go cache returned access denied in the review session; this temporary cache worked.

## Verified implementation and fixes

- CLI now resolves the OS save folder or `--save-dir`, creates a store, offers Continue when files exist, and saves the initial state when starting in an empty folder. It prints the save-and-quit note after restoring the terminal and releases the final model's lock.
- Continue releases the startup claim before displaying the list, then claims the selected run on adoption. Previously it tried to claim a lock it already held, preventing Continue, recovery, and interrupted keyboard replay.
- Accepted buy/sell and loan actions now close their input box even if autosaving fails. Acceptance is distinguished from save failure by recorded action count; engine refusals still leave input editable.
- Failed saving gets a persistent NOT ON DISK warning. Save error retry remains S from the board; q allows leaving without claiming a successful save.
- Recovery records the damaged active file's checksum so a deliberate repair is allowed. Saving does not replace a valid backup with a corrupt active file. Backups are copied instead of renaming away the active file before replacement; lock heartbeat errors are propagated.
- Save schema 1, rules version prototype-1, and content version campaign-1 are explicit and validated on load. Missing or unsupported versions are refused with an explanation. There is no automatic migration; versionless experimental saves must be kept for a matching build or replaced explicitly. Build version remains diagnostic metadata.
- New Run from the ending changes the seed-based save slot and releases the old lock. README now describes the playable prototype, persistence, save directory, and actual quit/export controls.

## Evidence from this session

All commands used the writable temporary Go cache:

- `go test ./... -count=1`: passed all packages after the fixes and new tests.
- `go test -race ./... -count=1 -timeout 300s`: passed all packages after the final code changes.
- `go vet ./...`: passed after the final code changes.
- `go build` to a temporary executable and `--help`: passed; help includes --save-dir.
- `gofmt -l cmd internal`: empty; `git diff --check`: passed after normalizing README line endings.
- New tests reject unsupported schema/rules/content versions and verify initial startup saving followed by Continue without a premature lock.
- Existing interrupted keyboard save/resume test now reaches the same $1,042 ending as the uninterrupted reference run.

## Next owner and stopping conditions

The local LLM should review the complete dirty diff, then collect the remaining
Phase 3/4 platform evidence and record it in TUI-TASKS.md. Keep Phase 5 work out
of this continuation until the earlier gates are satisfied or a human explicitly
changes the plan. Version changes require an explicit compatibility/migration
decision; do not silently accept older files.

## What I could not verify

No real macOS host was available. No human-driven terminal save/quit/resume run,
OS interruption during replacement, or external multi-process stress run was
performed here. Automated keyboard and save tests establish their tested paths,
not all OS interruption guarantees. Review stale-lock takeover and concurrent
writer races before claiming the persistence platform gate. Starting a fresh
run from the Continue list deliberately preserves the old save until the first
accepted action; verify that behavior against the plan's initial-save policy
before closing Phase 4. These are remaining gate obligations, not passing claims.

## Corrections to the previous handoffs

1. Phase 3 was already committed in 021c1c5 with evidence in 821ea52; the original handoff's uncommitted Phase 3 instructions were stale.
2. The orientation review correctly found six failing TUI save tests, missing CLI wiring, and absent version contracts. Those failures and integration gaps have now been addressed and the full suite passes.
3. Several test assertions were wrong: borrowing $5,000 adds to the initial $20,000 debt, rather than replacing it. Receipt assertions now normalize layout whitespace while preserving their numeric expectations. Continue-list isolation is represented by a nil run until adoption.
4. Initial review did not establish the failure causes. This session traced and fixed them; do not attribute all failures to test defects, because lock ownership, recovery, warning visibility, and overlay behavior also required production fixes.
