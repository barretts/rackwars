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

Optional content remains outside the base-release gates.