# RACK WARS — TUI execution checklist

Detailed plan: [TUI-PLAN.md](TUI-PLAN.md). Rules and balance targets: [RACK-WARS.md](RACK-WARS.md).

All tasks start unchecked. A checkbox means the work and its required evidence exist, not merely that the design mentions it. Record the commit/build, seed or trace, test command or terminal environment, result, and any remaining gap when closing a phase.

## Phase 1 — Terminal foundation

- [ ] Initialize Go module; pin toolchain and compatible Bubble Tea/Lip Gloss v2 dependencies.
- [ ] Build the market-shaped terminal shell, selection, overlay, help, and clean exit.
- [ ] Support 80×24, wider layouts, resizing, ASCII, no-color, and non-TTY errors.
- [ ] Verify key handling and terminal restoration in macOS Terminal.app and Windows Terminal.
- [ ] Record OS minimums and architecture coverage; add native CI tests/builds when available.
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

Optional content remains outside the base-release gates. See the detailed plan before adding another feature or changing an acceptance criterion.
