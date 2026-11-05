# RACK WARS — TUI execution checklist

Detailed plan: [TUI-PLAN.md](TUI-PLAN.md). Rules and balance targets: [RACK-WARS.md](RACK-WARS.md).

All tasks start unchecked. A checkbox means the work and its required evidence exist, not merely that the design mentions it. Record the commit/build, seed or trace, test command or terminal environment, result, and any remaining gap when closing a phase.

## Phase 1 — Terminal foundation

- [x] Initialize Go module; pin toolchain and compatible Bubble Tea/Lip Gloss v2 dependencies.
- [x] Build the market-shaped terminal shell, selection, overlay, help, and clean exit.
- [x] Support 80×24, wider layouts, resizing, ASCII, no-color, and non-TTY errors.
- [ ] Verify key handling and terminal restoration in macOS Terminal.app and Windows Terminal.
- [x] Record OS minimums and architecture coverage; add native CI tests/builds when available.
- [ ] Close the platform gate with interactive evidence for both primary operating systems.

## Phase 2 — Trading engine

- [ ] Define deterministic state, validated actions, integer money/power, and content IDs.
- [ ] Implement three markets and three products with finite stock and stable morning quotes.
- [ ] Implement buying, selling, borrowing, repayment, capacity, freight, and interest.
- [ ] Implement margin cures, forced sales, surplus proceeds, and insolvency.
- [ ] Add price anchors, 30-day settlement, score, and headless action traces.
- [ ] Pass rule tests and record a winning trading trace and a warned leveraged-loss trace.

## Phase 3 — Playable trading TUI

- [ ] Connect board selection, quantity entry, buy/sell, lender, travel, and Stay to the engine.
- [ ] Show costs, debt exposure, capacity, known quotes, headlines, and tomorrow's warning.
- [ ] Add invalid-action explanations, cancellation, and protection against duplicate day commitments.
- [ ] Add receipts, ending, New Run, and Replay This Seed.
- [ ] Complete a keyboard-only 30-day run on Windows and macOS.
- [ ] Close the trading prototype gate without hosting or jackpot subsidies.

## Phase 4 — Persistence and replay

- [ ] Implement versioned save/load, Continue, autosave, and save-and-quit.
- [ ] Use OS-appropriate directories and a `--save-dir` override.
- [ ] Preserve pending choices, random state, content version, and summary statistics.
- [ ] Implement last-good backup, tested file replacement, recovery, and visible write failures.
- [ ] Prevent concurrent writers from silently overwriting an active run.
- [ ] Verify uninterrupted versus resumed traces, corrupt/incompatible saves, and both OS path behaviors.

## Phase 5 — Full GPU game

- [ ] Add the remaining three products and two markets.
- [ ] Implement leases, power selection, electricity, ordinary hosting, and featured workloads.
- [ ] Implement heat, customs choices, recovery, and regional constraints.
- [ ] Complete the campaign, readable warnings, eligibility, satire, and reconciled receipts.
- [ ] Update save versions and extend rule/replay coverage.
- [ ] Record successful cautious, hosting, and risky-import runs without mandatory bonus contracts.

## Phase 6 — Balance and hardening

- [ ] Run and retain the 20-seed benchmark results from the game design.
- [ ] Tune debt, spreads, stock, event strength, and ranks using recorded outcomes.
- [ ] Observe five first-time players and record comprehension, duration, and replay behavior.
- [ ] Test minimum/wide/resized terminals, long numbers/headlines, ASCII, and no-color.
- [ ] Verify focus, repeated keys, Esc, Ctrl+C, paused choices, and terminal restoration.
- [ ] Close the release-candidate gate with passing checks and explicit platform evidence.

## Phase 7 — Native distribution

- [ ] Build macOS arm64/amd64 and Windows amd64 artifacts with embedded content.
- [ ] Prepare archives, permissions, version information, license notices, and checksums.
- [ ] Test extracted builds offline on target systems with no Go installation.
- [ ] Verify launch from paths with spaces, save/quit/resume, and a completed run.
- [ ] Document launch commands, controls, save locations, OS requirements, and bug-report traces.
- [ ] Record signing status and any untested architecture; prepare repeatable release automation.
- [ ] Close the release gate only when each required target has execution evidence.

## Evidence record

Copy this entry for each completed phase or unresolved gate:

```text
Phase:
Owner:
Commit/build and engine/content version:
OS / architecture / terminal:
Commands, seed, or action trace:
Observed result:
Open gaps and next owner:
Gate: passed / incomplete
```

```text
Phase: 1 — Terminal foundation
Owner: omp session (expert-engineer-peirce)
Commit/build and engine/content version: commit 9ab28b9 on branch main (first
  commit of the repository). Binary `rackwars 0.1.0-phase1`, built with
  `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=0.1.0-phase1"
  -o dist/rackwars.exe ./cmd/rackwars`. Dependencies pinned by go.mod
  (`go 1.26.0`): charm.land/bubbletea/v2 v2.1.1, charm.land/lipgloss/v2 v2.0.6,
  github.com/charmbracelet/x/ansi v0.11.9, github.com/charmbracelet/x/term v0.2.2.
OS / architecture / terminal: Windows 11 build 26200, x64 (GOAMD64=v1 default).
  Interactive runs were driven through a ConPTY pseudo-terminal created by the
  agent harness (Git Bash/MSYS parent), which is the same ConPTY path Windows
  Terminal uses, but the Windows Terminal UI itself was not driven. No macOS host
  is reachable from this environment, so no darwin evidence exists.
Commands, seed, or action trace:
  Static: `gofmt -l .` (no output); `go vet ./...` (clean);
  `go test -race ./... -count=1 -timeout 300s` -> ok rackwars/internal/game,
  ok rackwars/internal/tui, no race reports.
  Interactive key trace (no seed; the shell has no engine yet), run as
  `./dist/rackwars.exe --ascii` inside ConPTY: `\x1b[B` (selection moved to the
  4090 row), `?` (help painted), a lone `\x1b` (help closed), `b` (quantity box
  opened), `12` typed (over-limit refusal painted *inside* the box), `q` (did
  NOT quit while the box had focus), `\x1b` (cancelled), `\r\r` (day never
  advanced; "Day 2/30" was never painted), Ctrl+C -> exit code 0 and the
  alt-screen exit sequence was emitted (terminal restored).
  Colour-off: `NO_COLOR=1 ./dist/rackwars.exe --no-color` inside ConPTY -> full
  board painted, Ctrl+C -> exit code 0.
  Non-TTY: `echo hi | ./dist/rackwars.exe` -> "RACK WARS needs an interactive
  terminal. Run it from Terminal.app, iTerm2, Windows Terminal, or PowerShell."
  and exit code 1; `--version` -> `rackwars 0.1.0-phase1` exit 0; `--help` ->
  usage exit 0; unknown argument -> usage exit 2.
  Scripted in-process (same key bytes, no PTY):
  `go test ./internal/tui -run TestScriptedKeysDriveTheShell` feeds real bytes
  through os.Pipe() into tea.WithInput, renders through the real Bubble Tea
  renderer, and rebuilds the visible screen grid (cursor moves, EL/ED, CR/LF)
  before asserting; it also asserts the rebuilt grid equals ansi.Strip(View())
  row by row.
Observed result: 80x24 monochrome frame occupies 13 lines and stays within 80
  cells; the 120-column run paints the side panel (DETAIL / HOLDINGS / SHORELINE
  CREDIT / TRAVEL) and truncates with `~` in ASCII mode; Unicode rules and the
  `»` marker and their ASCII `-` / `>` replacements both render correctly; every
  frame stays under 80 SGR escape pairs (TestFrameStaysWithinEscapeBudget, added
  after measuring that Lip Gloss v2 emits one SGR pair per grapheme when
  Bold+Underline are combined, so the header style uses Reverse+Bold instead);
  refusal reasons render inside the quantity box because the box covers the
  board status line; `q` is not a shortcut while quantity entry has focus; two
  Enters never commit two days; Ctrl+C exits 0 with the terminal restored.
Open gaps and next owner:
  1. No macOS interactive evidence. Next owner must replay the key trace above in
     Terminal.app on darwin/arm64 and on an Intel (darwin/amd64) Mac, and record
     the terminal restoration check there.
  2. `.github/workflows/ci.yml` is authored (windows-latest + macos-latest: vet,
     gofmt, CGO_ENABLED=0 build, tests, race tests on macOS, `--version` smoke)
     but has never executed: this repository has no remote and no Actions run
     history. Next owner pushes it and records the run URL.
  3. Resizing is covered only by synthesized `WindowSizeMsg` (narrow/wide switch)
     and never by a real SIGWINCH from a resized window.
  4. Key handling was verified in a ConPTY pseudo-terminal, not in the Windows
     Terminal application window.
Gate: incomplete
```

Optional content remains outside the base-release gates. See the detailed plan before adding another feature or changing an acceptance criterion.
