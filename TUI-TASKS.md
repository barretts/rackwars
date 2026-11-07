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

- [x] Define deterministic state, validated actions, integer money/power, and content IDs.
- [x] Implement three markets and three products with finite stock and stable morning quotes.
- [x] Implement buying, selling, borrowing, repayment, capacity, freight, and interest.
- [x] Implement margin cures, forced sales, surplus proceeds, and insolvency.
- [x] Add price anchors, 30-day settlement, score, and headless action traces.
- [x] Pass rule tests and record a winning trading trace and a warned leveraged-loss trace.

## Phase 3 — Playable trading TUI

- [x] Connect board selection, quantity entry, buy/sell, lender, travel, and Stay to the engine.
- [x] Show costs, debt exposure, capacity, known quotes, headlines, and tomorrow's warning.
- [x] Add invalid-action explanations, cancellation, and protection against duplicate day commitments.
- [x] Add receipts, ending, New Run, and Replay This Seed.
- [ ] Complete a keyboard-only 30-day run on Windows and macOS.
- [ ] Close the trading prototype gate without hosting or jackpot subsidies.

## Phase 4 — Persistence and replay

- [x] Implement versioned save/load, Continue, autosave, and save-and-quit.
- [x] Use OS-appropriate directories and a `--save-dir` override.
- [x] Preserve pending choices, random state, content version, and summary statistics.
- [x] Implement last-good backup, tested file replacement, recovery, and visible write failures.
- [x] Prevent concurrent writers from silently overwriting an active run.
- [ ] Verify uninterrupted versus resumed traces, corrupt/incompatible saves, and both OS path behaviors.

## Phase 5 — Full GPU game

- [x] Add the remaining three products and two markets.
- [x] Implement leases, power selection, electricity, ordinary hosting, and featured workloads.
- [x] Implement heat, customs choices, recovery, and regional constraints.
- [x] Complete the campaign, readable warnings, eligibility, satire, and reconciled receipts.
- [x] Update save versions and extend rule/replay coverage.
- [x] Record successful cautious, hosting, and risky-import runs without mandatory bonus contracts.

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

```text
Phase: 2 - Trading engine
Owner: omp session (expert-engineer-peirce)
Commit/build and engine/content version: commit d7ac9cc on branch main, on top of
  9ab28b9 and 5973d2d (Phase 1). Engine content version: internal/content tables
  (3 prototype markets: garage, ashburn, westtexas; 3 prototype products:
  3090sled, 4090, h100) plus the 30-day campaign table, validated by
  content.Validate. Binary `rackwars 0.1.0-phase2`, built with
  `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=0.1.0-phase2"
  -o dist/rackwars.exe ./cmd/rackwars`; the shipped binary is still the Phase 1
  shell (the engine is not wired into the TUI until Phase 3), so `--version`,
  `--help`, and the non-TTY guard are the only new-binary checks here.
OS / architecture / terminal: Windows 11 build 26200, x64 (go1.26.0
  windows/amd64, GOAMD64=v1, CGO_ENABLED=0). Phase 2 is headless: the surface is
  `go test`, no terminal interaction required. Git Bash/MSYS parent shell. No
  macOS host reachable, so no darwin test run exists.
Commands, seed, or action trace:
  Static: `gofmt -l .` (no output); `go vet ./...` (clean).
  `go test ./... -count=1` -> ok rackwars/internal/content (21 tests),
  ok rackwars/internal/game (25 tests), ok rackwars/internal/sim (6 tests),
  ok rackwars/internal/tui (34 tests).
  `go test -race ./... -count=1 -timeout 300s` -> same four packages ok, no race
  reports.
  Winning trade-only trace, recorded in the repository and replayed from disk:
  `testdata/traces/trade-repay-win.json` (+ `.txt`, `.golden.json`) - seed 1,
  policy trade-repay, 34 actions, cast 3090sled/4090/h100, markets
  garage/ashburn/westtexas, prototype rules. Check it with
  `go test ./internal/sim -run TestRecordedTracesStillReplay -count=1`.
  Warned leveraged-loss trace: `testdata/traces/leveraged-crash.json`
  (+ `.txt`, `.golden.json`) - seed 1, policy leveraged, 10 actions.
  Balance table: `go test ./internal/sim -run TestPoliciesFinishEveryRun
  -count=1 -v` prints 12 seeds x 3 policies (trade-repay, hold, leveraged).
Observed result:
  Trade-and-repay finish (TUI-PLAN gate 2, first half): seed 1 buys nothing until
  day 2's headline warns "A merger announcement tomorrow is expected to call GPUs
  finished"; on day 3 the announced crash lands in West Texas (3090 sled ask
  $327, i.e. 27% of that town's usual midpoint) and the policy buys 12 units for
  $3,924 and pays $4,576 on the note the same morning. It holds through days 4-6
  while the balance compounds, sells all 12 on the morning of day 7 for $28,776
  gross (about 2.0x the usual midpoint), repays the remaining $28,734, and closes
  on day 30 with cash-out $1,042, borrowed $0, repaid $33,310, interest paid
  $13,310, zero forced sales. `TestGateTradeAndRepayFinish` fails unless the win
  has Borrowed == 0, Repaid > 0, and Interest > 0, so a jackpot- or credit-
  subsidised finish cannot satisfy the gate.
  Warned leveraged crash (second half): seed 1 under the leveraged policy borrows
  $14,468, buys 8 4090s, and is forced to sell 8 units across days 5, 7 and 8
  (recorded loss $942, interest paid $24,897), ending repossessed on day 8 with
  $32,630 owed. `TestGateWarnedLeveragedCrash` asserts a non-empty Tomorrow line
  on the morning before every forced-sale day in the run, that the player had
  borrowed, and that the forced sales actually cost money. The fall that broke
  this run was announced the morning before it landed (day 2 Tomorrow), and the
  rebound that would have saved it was announced too (day 5 Tomorrow: "A chatbot
  launch is expected to create demand for ordinary consumer cards.").
  Reproducibility (third half): the recorded JSON holds only the config and the
  action list. `TestRecordedTracesStillReplay` decodes both fixtures, replays
  them, and compares cash/debt/net worth/ending/statistics against the sibling
  `.golden.json`; `TestSameSeedSameActionsProduceIdenticalRuns`,
  `TestDifferentSeedsGiveDifferentBoards`, `TestPreviewsArePureAndExact` and
  `TestTraceRoundTripReproducesRun` cover the same property in-engine, and
  `internal/sim.Play` re-derives every step log by replaying the actions against
  a fresh run, so a trace that is quoted as evidence is always re-runnable.
  Worked debt examples (fourth half): `TestUntouchedNoteGrowsTenPercentPerDay`
  reconciles the $20,000 starter note to $22,000 and $24,200 after one and two
  days; `TestInterestRoundsUpToNextDollar`; `TestBorrowIsBoundedByCollateral`;
  `TestForcedSaleWorkedExample` reproduces the design's two-H100 shock exactly
  (bids $40,000 -> $16,000, collateral $32,000, ceiling $46,000, breach $14,000,
  two forced sales, $28,000 debt, $0 cash, no repossession);
  `TestCashCureHappensBeforeForcedSale`, `TestSaleProceedsRouteToLenderFirst`
  (whole-unit surplus returned to cash), `TestRepossessionOnlyWhenNothingLeft`
  and `TestRefusalsCarryExactGaps` cover the rest of the credit section.
  Balance table over 12 seeds: trade-repay reaches the day-30 close on 8 seeds
  and finishes with a positive cash-out on 3 (seeds 1, 2, 5); hold is repossessed
  on all 12 (days 10-17); leveraged is repossessed on all 12 (days 4-9) with 8-10
  forced units each. Every run ends in a terminal state - no run hits the
  400-action budget.
  Defects found and fixed while producing this evidence:
  1. `content.CampaignFor` assigned `h.Effects = nil` before ranging over
     `h.Effects`, so every campaign effect was dropped and no campaign price move
     ever applied in any run. Fixed and covered by
     `TestCampaignForPrunesForeignProducts` and `TestEveryBigCampaignMoveIsAnnounced`.
  2. The day-30 closing auction settled a positive cash-out by subtracting it from
     debt, leaving a negative debt balance that the ending screen would have shown
     as the lender owing the player money. Settlement now zeroes the note and hands
     the remainder back as cash; `TestDay30ClosesTheRun` pins it, and
     `internal/sim.Play` polices cash >= 0, debt >= 0, and slot capacity on every
     driven step.
  3. Content validation could not see campaign effects pointing at products or
     markets outside the cast; Validate now checks the full campaign table and
     rejects dangling IDs, and CampaignFor prunes effects that are legal in the
     table but foreign to the cast.
Open gaps and next owner:
  1. The winning margin is thin: the best of twelve seeds clears only $1,042 after
     paying $13,310 of interest, and this policy sits flat for 23 days because it
     only buys at 50% or less of a town's usual midpoint. The trading economy is
     provably winnable without hosting or campaign jackpots, but the spread
     versus the 10%/day note needs Phase 6 tuning; the balance table above is the
     starting data for that work.
  2. No darwin run of `go test ./...` exists (no macOS host reachable). The
     packages are pure Go with no cgo or OS-specific code, but the next owner
     should record the macos-latest CI result before the Phase 7 release gate.
  3. `.github/workflows/ci.yml` still has never executed (no remote, no Actions
     history). Next owner pushes and records the run URL.
Gate: passed
```

```text
Phase: 3 - Playable trading TUI
Owner: omp session (expert-engineer-peirce)
Commit/build and engine/content version: commit 021c1c5 on branch main, on top of
  a1bcf2c, d7ac9cc, 5973d2d and 9ab28b9 (Phases 1-2). Content version unchanged
  from Phase 2: internal/content prototype cast (garage, ashburn, westtexas;
  3090sled, 4090, h100) plus the 30-day campaign, validated by content.Validate,
  with hosting, electricity, power, heat, encounters, allocations and operating
  leases still switched off by content.PrototypeRules(). Binary
  `rackwars 0.1.0-phase3`, built with
  `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=0.1.0-phase3"
  -o dist/rackwars.exe ./cmd/rackwars`; this is the first binary whose TUI is
  wired to the engine.
OS / architecture / terminal: Windows 11 build 26200, x64 (go1.26.0 windows/amd64,
  GOAMD64=v1, CGO_ENABLED=0). Interactive runs were driven inside a ConPTY
  pseudo-terminal started by the agent harness (about 120 columns wide, so the
  wide layout with the DETAIL panel), parent shell Git Bash/MSYS. No macOS host
  is reachable from this machine, so no darwin run of the game or of `go test`
  exists.
Commands, seed, or action trace:
  Static: `gofmt -l .` (no output); `go vet ./...` (clean).
  `go test ./... -count=1` -> ok rackwars/internal/content (10 tests),
  ok rackwars/internal/game (25 tests), ok rackwars/internal/sim (6 tests),
  ok rackwars/internal/tui (44 tests).
  `go test -race ./... -count=1 -timeout 300s` -> same four packages ok, no race
  reports.
  `go test ./internal/sim -run TestRecordedTracesStillReplay -count=1` -> ok: the
  Phase 2 fixtures `testdata/traces/trade-repay-win.{json,txt,golden.json}` and
  `leveraged-crash.*` still replay against the engine the TUI now drives.
  Engine-equivalence gate: `go test ./internal/tui -run TestScriptedKeyboard -v`
  -> PASS x2. Both tests drive the real Bubble Tea program (120x30 screen
  emulator, raw key bytes through an os.Pipe) using the action list that
  `internal/sim.Play` produced headlessly for seed 1, translated to keys by
  `keysFor`, and then require the painted ending to equal the engine's own
  `End`: `TestScriptedKeyboardRunWinsOnSeedOne` (trade-repay, day-30 close) and
  `TestScriptedKeyboardRunGetsRepossessedOnSeedOne` (leveraged, repossession).
  The key bytes are never hand-written in these tests, so a screen number that
  drifts from the engine fails the test instead of drifting with it.
  Live keyboard run, Windows ConPTY: `./dist/rackwars.exe --seed 1`, driven one
  action per stdin write. Accepted keys in order: `t` `1` enter (road 1 to
  Ashburn), `t` `2` enter (road 2 to West Texas), `b` `12` enter, `l` `r` `4576`
  enter, four Stay closes (`e` enter, `ee` enter, `enter` enter, `e` enter - the
  `e`, `enter` and letter aliases are all board bindings), `s` `12` enter,
  `l` `r` `28734` enter, twenty-four further Stay closes, then `q`. Ctrl+C was
  delivered separately to another instance of the same binary.
Observed result:
  The live run reproduces the recorded Phase 2 winning trace for seed 1 exactly.
  Day 1 in The garage: cash $10,000, debt $20,000, net -$10,000, 3090 SLED ask
  $1,756. Day 2 morning after the first travel: freight $250, interest $2,000,
  cash $9,750, debt $22,000, panel receipt labelled `DAY 1 CLOSE`. Day 3, in
  West Texas under `THE MERGE: GPUs ARE OVER`: 3090 SLED ask $327, bought 12 for
  $3,924 (HOLDINGS shows `3090 SLED x12 avg $327`, slots 24/24), repaid $4,576,
  leaving cash $1,000 and debt $19,624. Day 7: sold all 12 at bid $2,398 for
  $28,776 gross - the status line read `Sold 12 3090 SLED for $28,776; you kept
  $28,776; realized +$24,852`, which is the design's rule, not an omission: the
  lender is paid by the credit check, not automatically out of every sale
  (`TestCashCureHappensBeforeForcedSale`, `TestSaleProceedsRouteToLenderFirst`) -
  then repaid $28,734 by hand, giving cash $1,042 and debt $0. Day 30 close
  screen: `DAY 30: THE CLOSING AUCTION`, `CASH OUT: $1,042 (after the note is
  settled)`, `RANK: Still independent`, and the receipt block `borrowed $0`,
  `repaid $33,310`, `interest paid $13,310`, `trade profit +$24,852`, `best flip
  +$24,852`, `peak net $1,042`, `freight paid $500`, `leases paid $0`, `forced
  sales 0 units $0`, `days on road 2`. Every figure matches
  `testdata/traces/trade-repay-win.golden.json` for seed 1, so TUI, engine and
  the committed Phase 2 evidence agree, and the win needed no hosting income and
  no campaign jackpot.
  Replay: pressing `[r]` on the ending screen returned `Day 1/30` in The garage,
  and that replayed run's `DAY 1 CLOSE` receipt (interest $2,000, cash $10,000,
  debt $22,000) was identical to the first run's day 1, so the seed really does
  re-deal the same market; `[n]` deals seed+1 and is pinned in-process by
  `TestNewRunDealsANewSeed`.
  Exit paths: `q` from the board -> process exit 0; Ctrl+C -> process exit 0 with
  the terminal restored; `./dist/rackwars.exe --version` -> exit 0;
  `echo hi | ./dist/rackwars.exe` -> `RACK WARS needs an interactive terminal.`
  and exit 1. Chrome modes: `--ascii` painted with `-` rules, `>` marker and `~`
  truncation, exit 0; `NO_COLOR=1 ./dist/rackwars.exe --no-color` painted and
  exited 0; the byte-level color contract stays pinned by
  `TestNoColorSetsNoForegroundOrBackground`, `TestColorFrameUsesStyleSequences`
  and `TestFrameStaysWithinEscapeBudget`.
  In-process Phase 3 behaviour tests, all engine-backed and comparing against
  engine accessors rather than literals: `TestCommitNeedsConfirmation` (three
  Enters close exactly one day), `TestMorningActionsCostNoDay`,
  `TestOversizedQuantityIsRefusedWithTheExactGap` (the box stays open, the draft
  survives, and the engine's shortfall is printed inside the box),
  `TestBoxesCancelWithoutTouchingTheRun`, `TestDayCloseReceiptAppearsInPanel`,
  `TestMorningReceiptNamesTheDayThePlayerWokeIn`,
  `TestLenderPanelUsesTheDesignFormat`, `TestTravelBoxShowsKnownQuotes`,
  `TestTravelNumberKeysPickTheRoad`, `TestTravelAndStayChargeWhatThePreviewPromised`,
  `TestResizeNeverAdvancesRun`.
  Why travel got number keys: inside this ConPTY channel an arrow key never
  arrived - `t` ESC `[B` enter landed as `t` enter and travelled on the default
  road - and only one line terminator per stdin write is delivered, so the
  30-day run above was fed one action per write. A destination that can only be
  reached with an arrow key is therefore not a destination the keyboard
  guarantees, and the quantity box already offers letter shortcuts; roads are
  now labelled `[1]`/`[2]` and a number picks the road outright, with an
  out-of-range number refused inside the box (`There is no road number 9 from
  here; the roads are numbered 1 to 2.`).
  Defects found and fixed while producing this evidence:
  1. `confirmBox` called `content.HeadlineFor(day+1)`, which returns nil past day
     30, so the day-30 confirmation panicked - a crash every player would hit on
     the last morning. Nil guard added, and the day-30 box now says `CLOSE THE
     BOOKS?` with `close the run for good` as its footer.
  2. The morning status line printed `rec.Day+1`; `Run.CommitDay` stamps a close
     receipt with the morning that follows, so the line named a day the player
     had not reached (board said day 13, line said 14). Fixed and pinned by
     `TestMorningReceiptNamesTheDayThePlayerWokeIn`. The panel label keeps naming
     the day that actually closed via `dayLines(closing, rec)`; the engine's own
     stamping is untouched so the Phase 2 traces stay valid.
  3. `.gitignore` listed a bare `rackwars`, which matched the `cmd/rackwars`
     directory: the program's `main.go` had never been committed and the tree at
     HEAD did not build. Patterns are now root-anchored (`/rackwars`,
     `/rackwars.exe`) and `cmd/rackwars/main.go` is committed in 021c1c5. CI has
     never executed, which is why a source tree that cannot build stayed hidden.
Open gaps and next owner:
  1. The Phase 3 exit gate asks for a full run in each required OS environment.
     Only Windows has interactive evidence; the macOS Terminal.app run
     (`./rackwars --seed 1`, plus `go test ./...` on darwin) is missing because
     no macOS host is reachable here. The keyboard-only 30-day checklist row
     therefore stays open and the gate is recorded incomplete.
  2. The live run used a ~120-column terminal. The 80x24 minimum layout is
     covered by tests (every layout test runs at 80x24), but a live narrow
     keyboard run on a real window has still not been recorded.
  3. Resizing remains verified only through synthesized `WindowSizeMsg`; no real
     SIGWINCH/resize event has been observed by the running program.
  4. `.github/workflows/ci.yml` has never executed (no remote, no Actions run
     history); the next owner pushes and records the run URL.
  5. The repossession ending has not been reached in a live terminal, only
     through the scripted renderer (`TestScriptedKeyboardRunGetsRepossessedOnSeedOne`).
     A live losing run is worth recording alongside the win.
  6. Carried from Phase 2 for Phase 6: the trading win is real but thin - the
     best of twelve seeds clears $1,042 after paying $13,310 of interest.
Gate: incomplete
```

```text
Phase: 4 - Persistence and replay
Owner: omp session (expert-engineer-peirce)
Commit/build and engine/content version: commit 2ebe167 on branch main, on top
  of 021c1c5, 821ea52, a1bcf2c, d7ac9cc, 5973d2d and 9ab28b9 (Phases 1-3).
  Save format SchemaVersion 1, engine `prototype-1`, content `campaign-1`;
  saves are `run-<seed>.json` with `run-<seed>.json.bak` last-good copies and
  `run-<seed>.json.lock` claim markers (line 1 `pid N on <host>`, line 2 an
  RFC3339 heartbeat, stale after 45 minutes). Binary `rackwars 0.1.0-phase4`,
  built with `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w
  -X main.version=0.1.0-phase4" -o dist/rackwars.exe ./cmd/rackwars`. This
  entry is committed together with the save-layer fixes described under
  "Defects found" below, which the interactive runs here re-ran against.
OS / architecture / terminal: Windows 11 build 26200, x64 (go1.26.0
  windows/amd64, GOAMD64=v1, CGO_ENABLED=0). Interactive runs again used the
  ConPTY pseudo-terminal started by the agent harness (wide layout, Git
  Bash/MSYS parent); the Windows Terminal application window itself was not
  driven, and no macOS host is reachable, so no darwin evidence exists.
Commands, seed, or action trace:
  Static: `gofmt -l cmd internal` (no output); `go vet ./...` (clean);
  `git diff --check` (clean).
  `go test ./... -count=1` -> ok rackwars/internal/content, game, save
  (1.259s), sim, tui (3.127s). `go test -race ./... -count=1 -timeout 300s` ->
  same five packages ok, no race reports (tui 14.083s).
  New regression tests: `internal/save`
  `TestTakeoverWindowCannotOverwriteAFileThatMoved` and `internal/tui`
  `TestTakeoverOfAStaleMarkerIsSurfaced`; the first was verified to fail
  against the pre-fix claim path ("wanted a clobbered failure, got none")
  and pass after the fix.
  Multi-process stress (separate throwaway module under
  %TEMP%\rw-harness, `rwstress <dir> <mode> [gapms]` + `rwverify <dir>`,
  not checked in):
  - race5: window A `write 250` (20 writes, live marker) while a script
    overwrote A's marker with an aged ghost stamp at t+2s; window C
    `spinsame` -> CLAIMFAIL "... is already open by pid 21972 on
    file-cabinet; close that RACK WARS window...", A completed WROTE 19/20
    with its bytes intact: an externally aged marker cannot steal a run
    from a live window.
  - race6: window A `write1b` (day-1 position, then frozen heartbeat),
    marker aged externally (genuinely stale), window C `write 300` ->
    clean takeover, 20/20 writes, `rwverify` -> "OK seed-1 day 9 cash 0
    debt 31211 over repossessed"; a blind third window then claimed and
    was refused "already holds that run on day 9, further than day 2, so
    nothing was overwritten...", file unchanged.
  - Interrupted-replacement rerun after the fixes: `spinsame` plus a
    NtSuspendProcess watcher caught the writer mid-replacement 3/3 times
    (`SUSPENDED tmp=run-seed-1.tmp-<pid> size=2225`); the orphan tmp was
    never adopted by `List` and the active file was loadable at every
    kill point.
  Live binary scenario, seed 1, %TEMP%\rw-ev2 (rebuilt 0.1.0-phase4 exe,
  one key per ConPTY write):
  - rwE: startup -> day-1 board + `run-seed-1.json` + lock
    "pid 18648 on file-cabinet"; `S` -> quit note naming the path, exit
    code 0, lock removed, `.bak` created.
  - A dead window's marker was planted (line 1 "pid 99999 on
    ghost-machine", heartbeat 46 minutes old). rwF onto the same folder
    -> Continue list -> enter -> takeover: lock line 1 became
    "pid 13828 on file-cabinet" with a fresh heartbeat, and the board
    status read "Carrying on: seed 1, day 1. Took over a save marker
    another window left behind (took ..." (frame width truncates the
    tail). `e`+enter closed the day; the next window's list showed the
    autosave: "day 2/30 cash $10,000 debt $22,000 ... still open written
    2026-11-05T19:24:06-06:00".
  - rwG (second window, same folder and seed) -> enter -> refused in
    place: "That run cannot be opened: ...\run-seed-1.json is already
    open by pid 13828 o...", stayed on the list, `q` exit 0. rwF `q` ->
    exit 0, lock released; final `rwverify` -> "OK seed-1 day 3 cash
    10000 debt 24200 over open".
  Default-directory smoke, live: `./dist/rackwars.exe --seed 1` with no
  `--save-dir` -> first save + live lock under
  `%APPDATA%\rackwars\saves\` (verified via rwverify: "OK seed-1 day 1
  cash 10000 debt 20000 over open"), a day close wrote the day-2 file
  plus `.bak`, clean quit released the lock; the test files were deleted
  afterwards.
Observed result:
  One live writer per run: a second window is turned away naming the
  holder pid (live rwG, scripted race5, `TestSecondWindowIsTurnedAway`);
  takeover only happens on a genuinely stale heartbeat, and the takeover
  window says so on the board (live rwF,
  `TestTakeoverOfAStaleMarkerIsSurfaced`); a claimant - takeover or
  blind - cannot overwrite bytes that moved after its claim: the save is
  refused ("changed since this window last saved it, so nothing was
  overwritten", hash-verified byte-identical file, `.bak` intact,
  `TestTakeoverWindowCannotOverwriteAFileThatMoved`); replacement is
  tmp+rename with retry, so interrupted runs leave either the whole old
  or the whole new file (3/3 kill-mid-replacement catches,
  `TestInterruptedKeyboardRunCarriesOnToTheSameEnding`); every overwrite
  keeps the previous bytes in `.bak`, and the damaged-save flow reports
  the damage and can recover the backup
  (`TestDamagedSaveIsReportedAndRecoverable`, live `.bak` appearances);
  autosave failures are visible and retryable
  (`TestFailedAutosaveIsVisibleAndRetryable`); unknown schema versions are
  refused rather than half-read (`TestUnsupportedVersionsAreRefused`).
  Documented semantics: a takeover window may write over a file that has
  not moved since its claim - that is the point of taking over, and the
  old bytes live in `.bak`; only a file that moved post-claim is refused.
  Defects found and fixed while producing this evidence:
  1. A fresh claim did not baseline the bytes on disk, so a takeover
     window could silently overwrite a save that had moved since the
     claim (same-day variants included). `Hold` now records the existing
     hash on both claim paths; the regression test fails pre-fix, passes
     post-fix.
  2. The heartbeat re-touch truncated the marker to zero between truncate
     and rewrite, so a reader could catch an empty marker and deem a live
     run stale. `Touch` is now tmp+rename, atomically whole-marker.
  3. `Hold` could steal a marker that had only looked stale for an
     instant; it now re-reads after 100ms and, if the holder revalidated,
     refuses naming it (race5 shows this live: the externally aged marker
     was rewritten by the live holder before the stealer's second read).
  4. Stale-marker takeover was silent: the board never said it happened.
     The note now surfaces at all five claim sites (startup claim,
     re-claim on first save, Continue, new-run adopt, recover).
  5. The stress harness had swapped argument handling (`<mode> <dir>`
     vs the documented `<dir> <mode>`), which silently no-opped earlier
     scenario invocations. Consequence: the prior session's `race1` log
     claiming a blind same-day silent overwrite is discounted as
     evidence (harness artifact); the invariant itself was real, was
     fixed (defect 1), and is now proven by the unit regression and the
     live scenarios above.
  ConPTY harness quirks (input side, not product defects): an empty
  stdin write lands as two Enters, and a bare Enter closes the day
  immediately (`[enter] face morning` is a live binding), so two live
  windows closed one extra day from a doubled Enter; the product's
  duplicate-day-commit guard covers a single key event and was not
  bypassed. `q` once needed a repeat write for the same reason.
Open gaps and next owner:
  1. No macOS evidence of any kind. On a darwin host: `go test ./... -count=1`
     and `-race`; live save/quit/resume, second-window refusal and
     stale-marker takeover in Terminal.app saving to
     `$HOME/Library/Application Support/rackwars/saves`; a real resize
     (SIGWINCH) observation. Windows Terminal application-window
     interactive evidence is also still owed everywhere (ConPTY harness
     only).
  2. All live driving above was keystrokes scripted by the agent; the
     Phase 3/4 human keyboard-only 30-day run on both primary OSes
     remains open.
  3. Unchecked row 3: the format carries a `pending` field and `Load`
     refuses a file whose pending choice this build cannot present, but
     no prototype code writes a non-empty `pending` and the `pending`-kind
     refusal has no test. State, random stream, content version and
     statistics are proven; the pending half is why the row stays open.
  4. `SchemaVersion 1` is accepted exactly; there is no migrator yet
     (none is needed while v1 is the only version, Phase 5 will need one).
  5. `.github/workflows/ci.yml` has still never executed (no remote);
     next owner pushes and records the run URL.
  6. Cosmetic: a long takeover note can exceed the status line (frame
     truncates with the `...` marker), and a `NOT ON DISK`-prefixed save
     failure can push one row past the frame height; the lock-refresh
     failure after a successful data write can surface one frame as a
     lock-kind refusal although the bytes landed (the next autosave
     clears it).
  7. `continueRun` does not clear the restart flag the way `adoptRun`
     does, so the exotic `n` (new run) -> pick Continue sequence keeps a
     restart-flavoured save (one allowed backwards write) until the next
     normal save; noted, no behaviour change made.
Gate: incomplete
```

```text
Phase: 5 - Full GPU game
Owner: omp session (expert-engineer-peirce)
Commit/build and engine/content version: on top of commit ba07a91 on branch
  main; this entry is committed together with the Phase 5 code it records.
  Save format SchemaVersion 2, rules `full-1`, content `campaign-2`, engine
  version stamped `0.1.0-phase5`. Binary `rackwars 0.1.0-phase5`, built with
  `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w
  -X main.version=0.1.0-phase5" -o dist/rackwars.exe ./cmd/rackwars`. The save
  store now carries its first forward migration: a version 1 file loads, its
  header retargets to the current triple (the run replays from its own embedded
  config, so a prototype save stays a prototype run), and the next autosave
  writes the header forward; a half-migrated file is refused, as is an unknown
  `pending` kind or a pending tag that disagrees with the replayed state.
OS / architecture / terminal: Windows 11 build 26200, x64 (go1.26.0
  windows/amd64, GOAMD64=v1, CGO_ENABLED=0). As in Phases 3-4 the interactive
  runs were driven through the agent harness's ConPTY pty services (wide
  layout, Git Bash/MSYS parent), one action per stdin write; the Windows
  Terminal application window itself was not driven and no macOS host is
  reachable, so no darwin evidence exists.
Commands, seed, or action trace:
  Static: `gofmt -l cmd internal` (no output); `go vet ./...` (clean).
  `go test ./... -count=1` -> ok rackwars/internal/content (0.591s), game
  (0.676s), save (1.348s), sim (0.660s), tui (3.116s). `go test -race
  -timeout 300s ./... -count=1` -> the same five packages ok, no race reports.
  Full-game rule tests (internal/game/full_test.go):
  `TestHostingPreviewMatchesSettlement`, `TestExplicitSubsetOverMeterRefused`,
  `TestGarageMeterForcesPartialHost`, `TestElectricityRoundsUpOnly`,
  `TestCurtailmentOnlyInsideOfferWindow`, `TestClawbackFollowsEnterpriseHostingOnly`,
  `TestAllocationHeatGateAndOncePerRun`, `TestHeatGatesFeaturedWorkload`,
  `TestCustomsStopBlocksEverythingElse`, `TestContestCoinDoesNotReroll`,
  `TestSurrenderTakesTheNamedUnitOnly`, `TestFineShortfallBecomesDebt`,
  `TestEncountersAndPrototypeRulesStayAsleep`, `TestTraceCodecPreservesPhase5Actions`.
  Content: the validator refuses broken tables and campaigns, prunes foreign
  products for a campaign-for run, announces every big campaign move, and
  `TestPrototypeRulesAreAPublicSwitchboard` pins the prototype as a config
  choice, not a code fork.
  TUI (internal/tui/fullgame_test.go): `TestFullBoardCarriesTheNewLines`,
  `TestPrototypeKeepsItsBoardAndRefusesAllocation`,
  `TestNightPlanPreviewMatchesEngineAndClosesTheDay`,
  `TestPoweredSubsetPickerRoundTrip`, `TestAllocationWindowOpensOnlyWhenTheEngineSaysSo`,
  `TestStoppedCalendarOwnsEveryKey`, `TestStoppedRunSavesTaggedAndResumesOnTheAnswer`,
  `TestHelpCoversTheNewKeys`.
  Save (additions): `TestVersionOneSaveMigratesOnLoad`,
  `TestHalfMigratedV1IsRefused`, `TestUnknownPendingKindIsRefused`,
  `TestPendingTagMustMatchState`, `TestPendingEncounterSurvivesSaveReload`.
  Phase 2 gate seeds: the `--seed N` flag is documented as a deterministic deal
  (same seed, same deal) without being an advertised difficulty ladder; the
  Phase 2 winning traces stay byte-pinned by `TestRecordedTracesStillReplay`
  and were untouched by this phase's content work (the prototype config pins
  the Phase 1-3 cast, rule tables and campaign tables - campaign day 21, the
  only full-rule content on the prototype map, was re-tuned from the
  prototype's own trace evidence, and the traces still replay).
  Archetype sweep (`internal/sim`, full config = six products, five markets,
  `FullRules()`): `TestFullGateThreeArchetypesFinish` pins that all three
  named styles - `trade-repay-full`, `host-run`, `risky-import` - finish day
  30 `closed` with a positive cash-out on every seed of {5, 16, 18, 25, 27,
  33}. Across seeds 1..40 each policy closes 11 runs and repossesses 29, and
  the wins land on exactly those six seeds for all three styles, so no style
  wins on a private lucky pocket. `risky-import` never claims an allocation
  and `trade-repay-full` needs no featured workloads at all: the gate's "no
  mandatory bonus contract" half holds by construction of the policies.
  Fog-of-war caveat, recorded honestly: quotes are last-seen values, so an
  unvisited Huaqiangbei reads 0/absent and the import trips that produced
  these wins bought on a prior visit and crossed cargo-light - across the
  winning runs zero cargo-bearing border stops fired and no fine was paid.
  The customs/fine machinery is owned by the engine tests above and by the
  live stopped-calendar capture below, not by the sweep.
  Live binary runs, ConPTY services, saves under %TEMP%\rw-evid, rebuilt
  0.1.0-phase5 exe, one action per write (write `X` lands as X+Enter, an empty
  write lands as two Enters - the Phase 4 harness quirk, plus a new one: a raw
  ESC byte arrives glued to the trailing newline as alt+Enter, so boxes that
  can only be left with Esc cannot be left by this harness):
  - Seed 5 (first full-board capture): day-1 garage board with the
    `[five markets]` tag, all six product rows (H100 ask $45,056),
    `Meter:   0.00 of 2.00 kW on the meter`, key line
    `[b]uy  [s]ell  [l]ender  [a]llocation  [t]ravel  [e]/[enter] plan the night`,
    the hosting preview line, and the day-1 used-card-aisle-full warning.
    `e` opened `THE NIGHT AHEAD AT The garage` (lease/meter/heat line, [1] AUTO
    with the engine's cash-after, [2] POWER OFF "the rack cools", [3] curtail
    printed as the no-offer note `no offer standing here tonight` on day 1);
    `2`+enter committed the power-off plan; `DAY 1 CLOSE` panel (interest
    $2,000) and the status line `Stayed the night; interest $2,000; now day 2.`
    followed. `a` opened `ANDY'S ALLOCATION` with
    `No allocation window is open here today.`
    The import path `t4` (garage -> Huaqiangbei, freight $1,000 on the DAY 1
    CLOSE panel), `jjb1` (bought 1 4090-48 MOD for $2,316, heat 0 -> 10),
    `t4` (Huaqiangbei -> The Dalles): this crossing drew no customs beat -
    clean arrival, MOD hosted +$90/electric -$2 in the travel-night preview -
    which is what the sweep said seed 5 does; it is recorded as the
    no-encounter contrast case.
  - The sweep of the exact live pattern (travel Huaqiangbei, buy one MOD,
    travel The Dalles) over seeds 1..3000 put the first customs stops at
    seeds 94, 106, 130, 143, 211, 213, 217, 229 (throwaway probe, not checked
    in). Seed 94 replayed live exactly: `t4`, `jjb1` (1 4090-48 MOD for
    $2,538, heat 10), `t4` -> the calendar STOPPED and the frame carried
    `CUSTOMS: LEAVING HUAQIANGBEI`, `The manifest says 1 units crossed. They
    would like to make this go away.`, `fine: $200   cash you hold: $5,412`,
    `contest at 40% to take nothing`,
    `losing contests costs the $200 fine and nothing more`,
    `they point at one box: the 4090-48 MOD with the broken corner.`,
    `[p] pay it   [s] hand over the goods   [c] contest (40%)`, the board's
    quotes frozen at $0, the header still Day 2/30, and the status
    `STOPPED AT thedalles - a customs beat - answer below, the calendar is
    not moving.` `p` paid: cash $5,412 -> $5,212 (the $200 fine), the calendar
    moved 2 -> 3, and the run later saved with `S` -> exit code 0,
    `run-seed-94.json` plus `.bak` both carrying `schemaVersion: 2`.
  - Closing day 3 at The Dalles on the AUTO plan gave the night plan with
    `hosted +$90   electric -$2`, `power 0.60/8.00 kW` and the power-off
    cash-after $4,862 beside the AUTO $4,950, then the itemized
    `DAY 3 CLOSE` panel (`lease $350`, `interest $2,420`, `hosting +$90`,
    `electricity $2`, `cash $4,950 debt $26,620`) with the matching status
    line `Stayed the night; lease $350; hosted +$90; electric $2; interest
    $2,420; now day 4.` The meter line read `0.60 of 8.00 kW on the meter`
    (one MOD at 0.60 kW) and `0.00 of 0.00 kW` at Huaqiangbei, which has no
    power at all - the regional constraint doing its job on the board.
  - Prototype smoke: `./dist/rackwars.exe --prototype --seed 3` ->
    `[trading prototype]` tag, three product rows,
    `Hosting: off (prototype)` and no Meter line, key line without
    `[a]llocation` and with `[e]/[enter] face morning`, the plain
    `FACE THE MORNING?` confirm; `a` was a silent no-op as pinned by
    `TestPrototypeKeepsItsBoardAndRefusesAllocation` (the sentence
    `Andy's allocations belong to the five-market game.` is the full-mode
    guard and is pinned in-process; this harness cannot open a board key box
    without its trailing Enter, so the live frame shows the no-op path).
    Autosave wrote `run-seed-3.json` at schema v2; the session was stopped
    with Stop-Process after the autosave check (no clean-exit claim here -
    the full-mode `S` exit 0 above is the clean-exit evidence).
  - `./dist/rackwars.exe --version` -> `rackwars 0.1.0-phase5`.
Observed result:
  The exit gate - "a cautious trading run, a legitimate hosting run, and a
  risky import run can finish on documented seeds" - is met: all three
  archetypes close on {5, 16, 18, 25, 27, 33} with positive cash-out
  (`TestFullGateThreeArchetypesFinish`), the hosting economy itemizes on the
  live board (meter, lease, hosted line, DAY CLOSE panel), the customs
  encounter stops the calendar live, answers with full disclosures, and pays
  out to the ledger, and a stopped run saves tagged and resumes straight onto
  the answer (`TestStoppedRunSavesTaggedAndResumesOnTheAnswer` pins the
  round trip including the writer-lock refusal for a second live window).
  Prototype behaviour is unchanged where it should be: same board minus the
  five-market lines, encounters and hosting asleep under prototype rules
  (`TestEncountersAndPrototypeRulesStayAsleep`), Phase 2 traces replaying.
  Defects found and fixed while producing this evidence:
  1. `Model.snapshot()` never copied `m.full` into the render model, so on
     every real frame the five-market tag, the Meter line, the hosting
     preview and the `[a]llocation` key were dead code - the live board was a
     prototype board wearing a full game's keys. `b.Full = m.full` added.
  2. `Update()` had no dispatch cases for the stay-plan, powered-subset and
     allocation modes: the handlers existed but were unreachable, so `e` from
     the board fell into the prototype confirm path and a full run could
     never close a day with hosting at all. Cases added for all three modes.
     Both defects were invisible to the earlier prototype-shaped tests because
     none of them ever rendered a full-mode model.
  3. Two fullgame tests mis-asserted their own harness (not the product): the
     first window never recorded the pre-model history (now seeded the way
     resume records it), and the "second window resumes" case never put the
     first window down first - the writer lock then correctly refused the
     resume it wanted to demo; the test now releases like `[S]` does before
     the second window claims. The resumed window also needs the window-size
     message before it can render, like every real window.
Open gaps and next owner:
  1. No macOS evidence of any kind, unchanged from Phases 3-4; the platform
     gate still needs `go test ./... -count=1` plus a live
     save/quit/resume/encounter run in macOS Terminal.app, and Windows
     Terminal application-window evidence is still owed everywhere (ConPTY
     harness only). The human keyboard-only 30-day runs also stay open.
  2. No live 30-day archetype run end to end in the TUI: the wins are pinned
     headless (sim gate) and the surface is pinned live, but one full live
     archetype run per style should be recorded in Phase 6's balance pass -
     the harness can drive it (every key in the full flow is reachable with
     digit+Enter paths), it is only long.
  3. The customs-box capture paid the fine; surrender and contest live frames
     are not recorded (in-process only:
     `TestSurrenderTakesTheNamedUnitOnly`, `TestContestCoinDoesNotReroll`).
     The contest's frozen-coin rule (the coin is drawn when the box is dealt,
     answering cannot reroll it) is engine-pinned.
  4. Featured-workload frames (allocation claim, clawback receipt line,
     curtailment offer window) are pinned in-process
     (`TestAllocationWindowOpensOnlyWhenTheEngineSaysSo`,
     `TestClawbackFollowsEnterpriseHostingOnly`,
     `TestCurtailmentOnlyInsideOfferWindow`) but were not hit live: the seeds
     driven here had no open window on day 1 and the power-off offer never
     stood at those sites that night.
  5. The subset editor's interior could not be screenshotted live: any write
     ends with Enter, which is also the editor's commit key, and raw ESC
     arrives glued to that Enter as alt+Enter. The round trip
     (open -> commit -> engine) is pinned by `TestPoweredSubsetPickerRoundTrip`
     and the editor does open and commit live; only the intermediate frame is
     harness-unreachable. A harness that can send a lone Enter or a lone Esc
     would close this.
  6. `--prototype --seed N` with a saved full-rules file of the same seed
     picks the file (continue list) - the mode is a property of the run, not
     the flag; fine, but a "new run of seed X in mode Y" affordance does not
     exist and is not claimed.
Gate: incomplete
```

## Addendum - opening-economy change (owner directive, 2026-11-06)

```text
Change: internal/game/state.go New() now opens every run at Cash 30000 on the
unchanged 20000 note (was 10000/20000): fresh net worth is +$10,000 instead of
-$10,000. Owner rationale: the negative opening had no reason; the debt is the
pressure, the bank balance should not be a pre-dug hole. Credit rules are
untouched - the unsecured floor is $30,000 and the ceiling is 50% of
collateral above it, neither references opening cash, and every credit
assertion in internal/game passed unmodified. RACK-WARS.md carries the new
opening line; the Phases 1-5 evidence blocks keep their original numbers as
history and are not rewritten here.

Blast radius and fixes:
  1. Tests that pinned the old literals were updated to the new position:
     internal/save (fresh-run header wants $30,000), internal/tui save tests
     (fresh cash 30000; borrow-5000 autosave frame now $35,000), and the
     scripted shell test's over-limit probe types 24 4090s - the deliberate
     cash-short refusal - because 12 became genuinely affordable at $30,000.
  2. Latent test-encoder defect found and fixed (the game itself was innocent,
     proven by an in-process replay of the exact reference position): the
     keyboard script builder keysFor assumed the board arrow cursor parked at
     row 0 before every buy/sell, but the board cursor is cyclic
     (m.sel = wrap(...)) and survives box commits and day closes. Every
     old-economy reference trace traded only row 0, so the assumption never
     moved; the new economy produced back-to-back row-2 H100 actions and the
     second encoder walk wrapped 2 -> 0 -> 1 and bought a 4090 instead,
     diverging from the reference trace mid-run. keysFor now threads the
     cursor position and walks the wrap correctly; the interrupted-resume
     test resets it at the cut because the resumed process legitimately
     restarts with the cursor on top (adoptRun). New fast pin:
     TestInProcessKeyboardScriptMatchesHeadless drives the whole reference
     trace in-process through the same encoder and must agree to the cent.
  3. The recorded prototype traces (testdata/traces) re-recorded on the new
     economy by the sim gate tests, as designed; TestRecordedTracesStillReplay
     re-validates them. The seed-1 prototype trade-repay win moved $1,042 ->
     $64,152 cash-out on the identical 36-action line.
  4. The 1..40 full-rules sweep fact in internal/sim/full_gate_test.go is
     superseded: at $30,000 all three archetypes close AND win on all 40
     seeds (was 11/40 close). The pinned six seeds {5,16,18,25,27,33} still
     pass unchanged as representatives; the comment records the new sweep.
     Difficulty re-balance is the owner's call, not silently retuned here.
  5. Live smoke (ConPTY harness, same caveats as Phase 5): rebuilt exe
     `rackwars 0.1.0-phase5.1`, fresh seed-1 full board paints
     "$30,000 cash  $20,000 debt  net $10,000  heat 0  slots 0/8" under the
     [five markets] header with Shoreline Credit ceiling $30,000 / headroom
     $10,000 / tomorrow $2,000 interest. `go vet`, `go test ./... -count=1`
     and `go test -race -timeout 300s ./... -count=1` all green.
Gate: unchanged - still incomplete for the same open gaps as Phase 5.
```

## Addendum - prose wrapping, slots-host-never-fence, card facts in the table (owner directives, 2026-11-06)

```text
Three owner-directed changes ship together.

A - wrapping contract ("never truncate my news again"): board prose wraps
word-by-word onto continuation lines (wrapFlow/wrapPad in internal/tui/view,
runewidth-aware), columns never do. Headline, warning, hosting/meter/alloc
lines, the status line, and the keys line all wrap; the table keeps its fixed
cell widths. When the window is too short, the keys line survives last and a
middle-tail notice counts the hidden wrapped lines ("N wrapped line(s) hidden
by the window height; resize to read them"). Pinned by TestProseWrapsInsteadOf
Truncating, TestFitHeightKeepsKeysVisible, TestWrapFlowCutsOverlongWords.

B - slots are hosting capacity, not an inventory fence. Owner: "you should be
able to buy unlimited cards, but only host the slots available, and you should
be able to move with the cards." Engine: BuyLimits fences a buy by cash and
shelf stock only; travel is never blocked by fleet size (arrival fence and the
storage-legality resume check deleted); freight keeps its bill, route fee plus
$50 x fleet slot footprint (SlotsUsed survives as that footprint). A hosted
night must fit BOTH the meter and the site's Market.Slots: stayPlan refuses an
explicit subset over the racks (ReasonOvercapacity, "over the site's slots"),
and AUTO packs the best-paying subset under both caps with an exact 2D 0/1
knapsack (hosting.go bestPowered; count-clipped items, deterministic recovery,
preview == settlement == replay). TravelPreview fields renamed to their new
meaning (FleetSlots, HostCap); StayPreview gained hostedSlots/slotCap. TUI
reframes: header "fleet N racks M", buy box "… 2 slots fleet 0" with the shop
wall only, travel option "hosts 16", night plan "slots 8" plus "hosting H/C
slots", the subset editor is "WHICH CARDS RUN TONIGHT?". Oversized saves are
legal resumes (illegal-save fence that rejected them deleted - it was a data-
loss bug). New pins: TestOversizedFleetStillTravels (16-slot fleet into the
garage: valid, footprint freight), TestAutoHostRackCapBindsBeforeMeter (20
mods on Ashburn's 16 racks: exactly 16, twice, identically), TestExplicitSubse
tRackBoundary (fill to the last rack; one more refuses without burning the
day; meter provably not the wall), TestBuyUnlimitedBySlotsShelfIsTheWall,
TestResumeAcceptsFleetLargerThanAnyTown (400 sleds resume).

C - the product table now carries the card details the hosting rule needs at
a glance: three new columns KW (per-unit draw), HOST/NGT (ordinary gross
hosting per night), CLASS (CON/ENT/MOD), taken straight from the content
definitions in snapshot(); pinned cell-for-cell at 80 columns by
TestProductTableCarriesCardFacts (row 79 chars, never re-wrapped by the prose
contract).

Blast radius: the recorded prototype traces re-recorded by the gate tests as
designed - the policy line is identical but unconstrained buying is visible
(seed-1 trade-repay buys 24 sleds where the old slot fence stopped it at 12,
carries a 48-52-slot fleet, sells into the spike, and cashes out $131,375 vs
$64,152). The 1..40 full-rules sweep was re-derived after the rule change:
all three archetypes still close AND win on all 40 seeds, so the pinned six
{5,16,18,25,27,33} stay and full_gate_test.go's comment records the new sweep.
RACK-WARS.md carries the rewritten capacity/freight/invariant lines.

Live smoke (ConPTY harness, same caveats as Phase 5): rebuilt exe
`rackwars 0.1.0-phase5.2`, fresh seed-1 full board paints the six-row table
with KW/HOST/NGT/CLASS ("H100 … 0.90 $350 ENT") and header "fleet 0 racks 8";
buying out the 16-sled shelf in the 8-rack garage is accepted ("the engine
will take 16 here"; fleet 32 racks 8); the night plan shows AUTO "power 1.60/
2.00 kW hosted +$160" with the tail wrapped to "hosting 2/8 slots"; the
oversized fleet then travels to Ashburn unblocked ("Traveled to Ashburn;
freight $1,850" = 250 + 32x50) where AUTO hosts 11.20 of 12.00 kW for +$1,120.
Long prose reaches the player whole and wrapped - the merger warning runs
"…Used-card prices" / "are likely to fall." across two board lines with no
ellipsis. `go vet`, `go test ./... -count=1` and
`go test -race -timeout 300s ./... -count=1` all green.
Gate: incomplete - the same open gaps as Phase 5 (ConPTY harness only; macOS
and human-play rows open).
```

Optional content remains outside the base-release gates.