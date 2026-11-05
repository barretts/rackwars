# RACK WARS

A GPU trading game for the terminal: buy accelerators cheap, find electricity,
pay Shoreline Credit's interest, and finish the 30-day closing auction solvent.

**Current status: playable trading prototype with persistence in development.**
Buy, sell, borrow, repay, travel, and close a 30-day run. Saves, Continue,
backup recovery, autosave, and trace export are wired into the terminal program.
The platform evidence gates in `TUI-TASKS.md` remain open.

## Requirements

| | Minimum |
|---|---|
| Windows | Windows 10 or Windows Server 2016 (the floor for Go 1.21 and later) |
| macOS | macOS 12 Monterey (the floor for Go 1.25/1.26; Go 1.27 will need macOS 13) |
| Terminal | 80 columns by 24 lines; wider terminals get a side panel |
| Toolchain | Go 1.26 (pinned by `go.mod`) |

Source: <https://go.dev/wiki/MinimumRequirements>. The game itself needs no
colour, no Unicode, and no mouse: `--no-color` and `--ascii` are tested modes,
not guesses about your terminal.

## Build

Windows PowerShell:

```powershell
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -X main.version=0.1.0" -o rackwars.exe .\cmd\rackwars
.\rackwars.exe
```

macOS (Terminal.app, iTerm2, any shell):

```sh
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=0.1.0' -o rackwars ./cmd/rackwars
./rackwars
```

Cross-builds from one machine are supported (`GOOS=darwin GOARCH=arm64` and
friends); a cross-built binary has to be run on its own OS to count as evidence.

## Running

```
rackwars [flags]

  --seed N     deal a specific seed (default 1)
  --save-dir PATH use a portable save folder
  --no-color   draw without color (NO_COLOR does the same)
  --ascii      draw with ASCII glyphs only
  --version    print the version and exit
  --help       show this help

Environment:
  NO_COLOR=1          same as --no-color
  RACKWARS_DEBUG=path append engine and terminal diagnostics to a log file
```

RACK WARS needs an interactive terminal. Piped or redirected stdin gets a one
line explanation and exit status 1; `--help` and `--version` work anywhere.

## Controls

| Key | Action |
|---|---|
| up/down, `j`/`k` | pick a product |
| `b` / `s` | buy or sell the selected product |
| `a` `b` `c` | quantity 1, 5, or the most that fits (in the quantity box) |
| digits | type an exact quantity |
| `l` | lender: borrow, repay, headroom, stress preview |
| `t` | travel: freight, capacity, risk |
| `e` | stay: host or power off |
| Enter | accept; committing a day needs a confirmation you have to answer |
| Esc | cancel; keeps the thing you were editing |
| `?` | help |
| `S` | save and quit; retry a failed save |
| `x` | export an action trace from the board or ending |
| `q`, Ctrl+C | quit; accepted actions are autosaved (Ctrl+C works from any screen) |

`q` is never a shortcut while the quantity box has focus: digits and letters are
input there, so typing a quantity cannot quit your run.

Saves live under `os.UserConfigDir()/rackwars/saves` (Windows AppData or macOS
Library/Application Support). Use `--save-dir` to override. Startup offers existing
runs, including damaged files with an explicit backup recovery choice. Unsupported
save schema, rules, or content versions are refused; no automatic migration exists.

## Development

```
go vet ./...
gofmt -l .
go test ./...              # engine rules, layouts, input handling
go test -race ./...        # needs a C toolchain (cgo) on Windows
```

Layout: `internal/game` owns the rules and all previews (money is integer only,
so the board can never disagree with the engine), `internal/tui` draws and reads
keys, `cmd/rackwars` parses flags and starts the program. The view never
computes a price, an interest charge, or a limit.

Tests drive the real Bubble Tea program: raw keyboard bytes through the input
reader, the real renderer, and an assertion layer that rebuilds the visible
screen grid, because the renderer only repaints changed cells.
