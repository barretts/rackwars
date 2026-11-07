# Agent guide

## Project and validation status

RACK WARS is a deterministic Go terminal trading game. The default executable starts the full six-product, five-market game; `--prototype` selects the earlier three-product, three-market trading rules. Go 1.26 is pinned in `go.mod`. The supported interactive targets are Windows and macOS, and shipped builds intentionally use `CGO_ENABLED=0`. The README's flag synopsis is behind the executable: consult `cmd/rackwars/main.go` for the current flags, including `--prototype`.

Before changing game behavior, check `RACK-WARS.md` for the intended rules and `TUI-TASKS.md` for open evidence/release gates. The game is implemented beyond some still-open interactive platform and balance evidence gates: do not mistake implemented functionality or passing CI for those gates being closed.

## Commands

```sh
go test ./... -count=1 -timeout 300s
go vet ./...
gofmt -l .
env CGO_ENABLED=0 go build -trimpath -o dist/rackwars ./cmd/rackwars
```

`go test -race ./... -count=1 -timeout 300s` is useful on hosts with a C toolchain; it is not required by Windows CI. CI tests/builds on Windows and macOS, runs vet on both, checks gofmt on non-Windows, runs race tests on macOS, and verifies `--version` without a terminal. A normal game launch requires an interactive stdin and stdout terminal. `--help` and `--version` do not.

## Architecture and invariants

- `cmd/rackwars` owns flags, terminal eligibility, logging setup, persistence startup, and Bubble Tea program lifecycle. The full game is the default; `--prototype` is explicit.
- `internal/content` is static product, market, campaign, and ruleset data plus validation. Stable string IDs are used in saves/traces; aligned state slices use indexes only internally.
- `internal/game` is the authoritative, terminal-independent rules engine. It owns integer-dollar money, deterministic seeded prices/events, previews, validation, action application, day resolution, and trace replay. Keep calculations here, never in the view. A player-facing preview and the committed action should share the same calculation path.
- `internal/tui` is the Bubble Tea v2 presentation/input layer. `Model` holds the run plus UI intent; `game.Run` owns game state. Board snapshots should derive from engine state/previews each frame, not be cached. Keep ASCII/no-color and the 80x24 minimum layout working.
- `internal/save` persists versioned JSON snapshots alongside the action trace that produced them. Load validates/replays the trace against the stored state; preserve deterministic outcomes and compatibility/version checks when changing state or content.
- `internal/sim` drives the engine headlessly with named deterministic policies. Policies should use only information available to a player (current and last-seen quotes, headline, warning, previews), not hidden campaign data or their own randomness.

The full game allows fleet inventory to exceed site capacity: slots constrain what can be hosted, not purchased, carried, or traveled with. Power is a separate hosting constraint. Travel carries the whole fleet. Preserve this distinction in rules, previews, and UI.

## Tests and debugging

Tests are colocated with their packages. Engine/content/save/simulation tests cover rules, deterministic replay, persistence and policies. TUI tests include ordinary model/view tests and real Bubble Tea program tests: they feed raw terminal key sequences, use an in-memory screen emulator, and verify rendered behavior. When a renderer test fails, inspect the emulated grid and viewport dimensions; raw output only repaints changed cells and is not a full frame.

For gameplay bugs, reproduce with `--seed N` (default seed is 1) and attach/export the action trace. `RACKWARS_DEBUG=path` enables appended engine/terminal diagnostics. Save files are under the OS user config directory at `rackwars/saves` unless `--save-dir PATH` is supplied; damaged saves have explicit recovery behavior. Do not assume every schema/rules/content version is migratable.

## Conventions to preserve

- Keep game currency as whole-dollar integer values. Power is represented in hundredths of a kW in content/engine data to avoid fractional arithmetic.
- Seed and action sequence determine the run. Do not add time-based or unseeded randomness to the engine; identical traces must replay identically.
- Keep user-facing explanations and action refusal reasons in the engine so the UI does not invent game rules.
- Use content IDs as stable persisted identifiers. Changes to persisted state, rules, or content require deliberate save/trace compatibility consideration and corresponding tests.
- Follow Go formatting (`gofmt`) and test package-specific changes first, then `go test ./...` and `go vet ./...` as appropriate.
