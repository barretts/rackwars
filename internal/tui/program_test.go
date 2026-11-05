package tui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// TestProgramRendersBoardAndRestoresScreen runs the real Bubble Tea program
// against an in-memory terminal at the minimum size. It proves the shell enters
// the alternate screen, paints the board through the actual renderer, and emits
// the exit sequence that gives the terminal back -- a View() string test cannot
// show any of that.
func TestProgramRendersBoardAndRestoresScreen(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := tea.NewProgram(testModel(NewChrome(false, true)),
		tea.WithContext(ctx),
		tea.WithInput(nil),
		tea.WithOutput(&out),
		tea.WithWindowSize(MinWidth, MinHeight),
		tea.WithColorProfile(colorprofile.ASCII),
	)
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(300 * time.Millisecond):
			cancel()
		case <-done:
		}
	}()

	if _, err := p.Run(); err != nil && !errors.Is(err, tea.ErrProgramKilled) {
		t.Fatalf("program run failed: %v", err)
	}
	close(done)

	frame := out.String()
	for _, want := range []string{"RACK WARS", "PRODUCT", "3090 SLED", "[b]uy"} {
		if !strings.Contains(frame, want) {
			t.Errorf("renderer output missing %q\n%q", want, frame)
		}
	}
	if !strings.Contains(frame, "\x1b[?1049h") {
		t.Errorf("renderer never entered the alternate screen\n%q", frame)
	}
	if !strings.Contains(frame, "\x1b[?1049l") {
		t.Errorf("renderer never left the alternate screen; the terminal would be left broken\n%q", frame)
	}
}

// TestScriptedKeysDriveTheShell feeds raw keyboard bytes through Bubble Tea's
// real input reader -- the same bytes a terminal sends, including the escape
// sequence for the down arrow -- and asserts on the reconstructed screen rather
// than on output bytes, because the renderer repaints only changed cells.
func TestScriptedKeysDriveTheShell(t *testing.T) {
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pipeRead.Close()
	defer pipeWrite.Close()

	const cols, rows = 100, 26
	scr := newScreen(cols, rows)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := tea.NewProgram(New(NewChrome(false, true), PreviewBoard()),
		tea.WithContext(ctx),
		tea.WithInput(pipeRead),
		tea.WithOutput(scr),
		tea.WithWindowSize(cols, rows),
		tea.WithColorProfile(colorprofile.ASCII),
	)
	runErr := make(chan error, 1)
	go func() { _, err := p.Run(); runErr <- err }()

	send := func(keys string) {
		if _, err := pipeWrite.Write([]byte(keys)); err != nil {
			t.Fatalf("send %q: %v", keys, err)
		}
	}
	mustWait := func(what string, pred func(string) bool) string {
		t.Helper()
		if !scr.waitUntil(4*time.Second, pred) {
			t.Fatalf("%s\nscreen was:\n%s", what, scr.text())
		}
		return scr.text()
	}

	// The emulator has to agree with the model's own view, otherwise every
	// assertion below is measuring the emulator instead of the game.
	mustWait("board never painted", func(s string) bool { return strings.Contains(s, "PRODUCT") })
	seeded := New(NewChrome(false, true), PreviewBoard())
	um, _ := seeded.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	wantLines := strings.Split(strings.TrimRight(ansi.Strip(um.(Model).View().Content), " \n"), "\n")
	gotLines := strings.Split(scr.text(), "\n")
	if len(gotLines) < len(wantLines) {
		t.Fatalf("screen has %d rows, the view has %d\nscreen:\n%s", len(gotLines), len(wantLines), scr.text())
	}
	for i, want := range wantLines {
		if strings.TrimRight(gotLines[i], " ") != strings.TrimRight(want, " ") {
			t.Errorf("row %d differs\nscreen: %q\nview:   %q", i, gotLines[i], want)
		}
	}

	// The down arrow arrives as the terminal's escape sequence.
	send("\x1b[B")
	mustWait("down arrow did not move the selection", func(s string) bool {
		line, ok := selectedLine(s)
		return ok && strings.Contains(line, "4090")
	})

	send("?")
	mustWait("? did not open help", func(s string) bool { return strings.Contains(s, "RACK WARS KEYS") })

	// Escape arrives on its own. A parser that saw ESC next to another byte
	// would read Alt+that key instead, so this also pins the sequence boundary.
	send("\x1b")
	mustWait("escape did not close help", func(s string) bool {
		return !strings.Contains(s, "KEYS") && strings.Contains(s, "[b]uy")
	})

	send("b")
	mustWait("b did not open the quantity box", func(s string) bool {
		return strings.Contains(s, "quantity:")
	})

	send("12")
	mustWait("typed digits did not reach the box", func(s string) bool {
		return strings.Contains(s, "quantity: 12")
	})

	send("\r") // accept an over-limit quantity
	mustWait("over-limit quantity was not refused in the box", func(s string) bool {
		return strings.Contains(s, "you need")
	})

	// q is a shortcut on the board, never while the quantity box has focus.
	send("q")
	select {
	case err := <-runErr:
		t.Fatalf("q quit while the quantity box had focus (run ended: %v)", err)
	case <-time.After(500 * time.Millisecond):
	}

	send("\x1b") // escape cancels the box
	mustWait("escape did not cancel the quantity box", func(s string) bool {
		return !strings.Contains(s, "quantity:") && strings.Contains(s, "[b]uy")
	})

	// Two Enter bytes arriving in one read must not spend two days.
	send("\r\r")
	mustWait("commit keys were not refused", func(s string) bool {
		return strings.Contains(s, "nothing changed")
	})
	if strings.Contains(scr.text(), "Day 2/30") {
		t.Errorf("a commit key advanced the day before the engine exists:\n%s", scr.text())
	}

	send("\x03") // ctrl+c always quits
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("ctrl+c ended the run with an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctrl+c did not end the run")
	}
}
