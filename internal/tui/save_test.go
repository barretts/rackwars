package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"rackwars/internal/content"
	"rackwars/internal/game"
	"rackwars/internal/save"
	"rackwars/internal/sim"
)

// A save slot is named after the seed, which is what makes --continue and a
// relaunch of the same command land on the same run.
const testRunID = "seed-1"

// saveStore returns a store in a temporary folder, with the folder repointed away
// from the user's real configuration directory.
func saveStore(t *testing.T) *save.Store {
	t.Helper()
	s, err := save.NewStore(t.TempDir(), "test")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

// sendRaw drives a keysFor-style script (arrow escapes, digits, enter) through the
// model as key messages, the way the terminal delivers the bytes.
func sendRaw(t *testing.T, m Model, script string) Model {
	t.Helper()
	r := []rune(script)
	for i := 0; i < len(r); i++ {
		var msg tea.KeyPressMsg
		switch {
		case r[i] == 0x1b && i+2 < len(r) && r[i+1] == '[':
			switch r[i+2] {
			case 'B':
				msg = key("down")
			case 'A':
				msg = key("up")
			default:
				t.Fatalf("script has an escape the driver does not know: %q", string(r[i:i+3]))
			}
			i += 2
		case r[i] == '\r':
			msg = key("enter")
		default:
			msg = key(string(r[i]))
		}
		um, _ := m.Update(msg)
		m = um.(Model)
	}
	return m
}

// savedAt plays the first n accepted actions of the reference seed-1 run through
// the real model with a store attached, and returns the model plus the reference
// steps. Everything happens through key presses, so what the save holds is what a
// player's hands put there.
func savedAt(t *testing.T, store *save.Store, n int) (Model, []game.Step, []game.Action) {
	t.Helper()
	ref, steps, err := sim.Play(sim.Prototype(1), sim.TradeRepay)
	if err != nil {
		t.Fatalf("reference run: %v", err)
	}
	if len(steps) < n+4 {
		t.Fatalf("the reference trace is too short to cut at %d: %d steps", n, len(steps))
	}
	_ = ref
	acts := make([]game.Action, 0, len(steps))
	for _, st := range steps {
		if st.OK {
			acts = append(acts, st.Action)
		}
	}
	m := sizedModel(t, NewChrome(false, true), 1, 120, 30).WithStore(SaveTarget{Store: store, RunID: testRunID})
	cur := content.Garage
	sel := 0
	done := 0
	for _, st := range steps {
		if !st.OK || done >= n {
			continue
		}
		m = sendRaw(t, m, keysFor(cur, st.Action, &sel))
		done++
		cur = st.Market
	}
	if m.saveErr != "" {
		t.Fatalf("autosave failed while playing %d actions: %s", n, m.saveErr)
	}
	if len(m.actions) != n {
		t.Fatalf("the run recorded %d actions, want %d", len(m.actions), n)
	}
	return m, steps, acts
}

// TestEveryAcceptedActionReachesDisk is the autosave contract: after each action
// the engine accepted, the file on disk holds that position, and its own history
// replays to it.
func TestEveryAcceptedActionReachesDisk(t *testing.T) {
	store := saveStore(t)
	m := sizedModel(t, NewChrome(false, true), 1, 120, 30).WithStore(SaveTarget{Store: store, RunID: testRunID})
	m = send(m, "l", "5", "0", "0", "0", "enter")
	if m.run.State.Debt != 25000 {
		t.Fatalf("the borrow did not land: %+v", m.run.State)
	}
	f, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("Load after a borrow: %v", err)
	}
	if !reflect.DeepEqual(f.State, m.run.State) {
		t.Errorf("the save does not hold the board on screen:\n got %+v\nwant %+v", f.State, m.run.State)
	}
	if len(f.Trace.Actions) != 1 {
		t.Errorf("the save carries %d actions, want 1", len(f.Trace.Actions))
	}

	// A closed day is a boundary too, and the file must not lag the screen.
	m = send(m, "e", "enter")
	f, err = store.Load(testRunID)
	if err != nil {
		t.Fatalf("Load after a committed day: %v", err)
	}
	if f.State.Day != m.run.State.Day || f.State.Day != 2 {
		t.Errorf("the save is on day %d, the screen is on day %d", f.State.Day, m.run.State.Day)
	}
	if len(f.Trace.Actions) != 2 {
		t.Errorf("the save carries %d actions, want 2", len(f.Trace.Actions))
	}
	// A refusal writes nothing: a save is only ever a position the player stood in.
	before, _ := store.Load(testRunID)
	m = send(m, "s", "9", "9", "9", "enter")
	after, _ := store.Load(testRunID)
	if !reflect.DeepEqual(before.State, after.State) {
		t.Errorf("a refused trade moved the save:\n before %+v\n  after %+v", before.State, after.State)
	}
}

// TestQuittingWithoutASaveFolderSaysSo covers a model that never got a store: the
// save keys must report that instead of pretending a write happened.
func TestQuittingWithoutASaveFolderSaysSo(t *testing.T) {
	m := sizedModel(t, NewChrome(false, true), 1, 120, 30)
	m = send(m, "S")
	if m.saveErr != "" {
		t.Errorf("a model with no store reported a save failure: %q", m.saveErr)
	}
	if !strings.Contains(statusLineOf(m), "not being written") {
		t.Errorf("no store, no message about it: %q", statusLineOf(m))
	}
}

// TestSaveAndQuitWritesThenLeaves is the [S] key: the run is on disk, the exit
// message names the file, and the folder is left open for the next window.
func TestSaveAndQuitWritesThenLeaves(t *testing.T) {
	store := saveStore(t)
	m := sizedModel(t, NewChrome(false, true), 1, 120, 30).WithStore(SaveTarget{Store: store, RunID: testRunID})
	m = send(m, "l", "5", "0", "0", "0", "enter", "b", "1", "enter")
	um, cmd := m.Update(key("S"))
	m = um.(Model)
	if cmd == nil {
		if _, err := store.Load(testRunID); err != nil {
			t.Fatalf("[S] did not write the run: %v", err)
		}
	}
	if cmd == nil {
		t.Fatalf("[S] did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("[S] did not produce a quit")
	}
	if !strings.Contains(m.QuitNote(), filepath.Base(saveFileName(store, testRunID))) {
		t.Errorf("the exit message does not name the file: %q", m.QuitNote())
	}
	f, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("Load after [S]: %v", err)
	}
	if len(f.Trace.Actions) != 2 {
		t.Errorf("the saved run has %d actions, want 2", len(f.Trace.Actions))
	}
	// The claim is given up, so the next window can take the run.
	if _, err := store.Hold(testRunID); err != nil {
		t.Errorf("the save is still claimed after quitting: %v", err)
	}
}

// TestFailedAutosaveIsVisibleAndRetryable covers the disk full / folder gone
// class: the board says it is not on disk, the run keeps playing, and [S] retries.
func TestFailedAutosaveIsVisibleAndRetryable(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a folder"), 0o600); err != nil {
		t.Fatalf("plant the blocking file: %v", err)
	}
	store := saveStore(t)
	// The folder became unwritable after the store opened: exactly what a removed
	// temp folder or a disconnected drive does to a running game.
	store.Dir = filepath.Join(blocker, "saves")
	m := sizedModel(t, NewChrome(false, true), 1, 120, 30).WithStore(SaveTarget{Store: store, RunID: testRunID})
	m = send(m, "l", "5", "0", "0", "0", "enter")
	if m.saveErr == "" {
		t.Fatalf("an unwritable folder did not report a save failure")
	}
	if !strings.Contains(frame(m), "NOT ON DISK") {
		t.Errorf("the board does not say the run is not on disk:\n%s", frame(m))
	}
	if !strings.Contains(statusLineOf(m), "S to try again") {
		t.Errorf("the failure sentence offers no retry: %q", statusLineOf(m))
	}
	// The run is untouched: a failed write must never damage the game.
	if m.run.State.Debt != 25000 || m.run.State.Cash != 35000 {
		t.Errorf("a failed save changed the run: %+v", m.run.State)
	}
	// [S] retries and, still failing, does not end the run.
	um, cmd := m.Update(key("S"))
	m = um.(Model)
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("[S] quit the run while the save was failing")
		}
	}
	if m.saveErr == "" {
		t.Fatalf("[S] claimed a save it could not make")
	}
	if !strings.Contains(frame(m), "NOT ON DISK") {
		t.Errorf("after a failed retry the board stopped warning:\n%s", frame(m))
	}
	// [q] still leaves, so a broken folder is never a trap.
	if !quits(m, "q") {
		t.Errorf("[q] would not leave a run whose save folder is broken")
	}
}

// TestSecondWindowIsTurnedAway covers two rackwars on one save folder: the second
// one says so, writes nothing, and stays playable.
func TestSecondWindowIsTurnedAway(t *testing.T) {
	dir := t.TempDir()
	first, err := save.NewStore(dir, "test")
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	second, err := save.NewStore(dir, "test")
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	m1 := sizedModel(t, NewChrome(false, true), 1, 120, 30).WithStore(SaveTarget{Store: first, RunID: testRunID})
	m1 = send(m1, "l", "5", "0", "0", "0", "enter")

	m2 := sizedModel(t, NewChrome(false, true), 1, 120, 30).WithStore(SaveTarget{Store: second, RunID: testRunID})
	if m2.saveErr == "" {
		t.Fatalf("a second window on the same run reported no problem")
	}
	if !strings.Contains(frame(m2), "NOT ON DISK") {
		t.Errorf("the second window does not warn that it cannot save:\n%s", frame(m2))
	}
	m2 = send(m2, "l", "5", "0", "0", "0", "enter")
	if m2.run.State.Debt != 25000 {
		t.Errorf("the refused save also broke the second run: %+v", m2.run.State)
	}
	// Only the first window's position is on disk.
	f, err := second.Load(testRunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(f.Trace.Actions) != 1 {
		t.Errorf("the second window wrote into the save: %d actions, want 1", len(f.Trace.Actions))
	}
}

// TestTakeoverOfAStaleMarkerIsSurfaced: after a crash, carrying on takes over
// the dead window's marker. The board has to say so, because the player cannot
// know whether some window still has the run open on screen: a takeover that
// looked identical to a normal resume would leave the second window's writes
// unexplained when they start being refused.
func TestTakeoverOfAStaleMarkerIsSurfaced(t *testing.T) {
	store := saveStore(t)
	m, _, _ := savedAt(t, store, 4)
	m = send(m, "q")
	path, err := store.Path(testRunID)
	if err != nil {
		t.Fatalf("path for %s: %v", testRunID, err)
	}
	old := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(path+save.LockSuffix,
		[]byte("pid 99999 on long-gone-machine\n"+old+"\n"), 0o600); err != nil {
		t.Fatalf("plant stale marker: %v", err)
	}
	// The start-up path: the list is offered with no claim taken, and carrying
	// on is where the stale marker gets taken over.
	started := sizedModel(t, NewChrome(false, true), 1, 120, 30).
		StartPersistence(SaveTarget{Store: store, RunID: testRunID})
	resumed := send(started, "enter")
	line := statusLineOf(resumed)
	for _, want := range []string{"Carrying on", "Took over a save marker another window left behind",
		"took over the save marker left by pid 99999 on long-gone-machine"} {
		if !strings.Contains(line, want) {
			t.Errorf("the takeover status is missing %q: %q", want, line)
		}
	}
	if resumed.run == nil {
		t.Fatal("carrying on after the takeover did not restore a run")
	}
}

// TestContinueListOffersSavedRunsAndNewRunLeavesThem checks the start-up prompt:
// it lists what is there, and starting a new run does not destroy it.
func TestContinueListOffersSavedRunsAndNewRunLeavesThem(t *testing.T) {
	store := saveStore(t)
	m, _, _ := savedAt(t, store, 6)
	m = send(m, "q")

	before, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("the run that was just played cannot be read back: %v", err)
	}
	wantDay := before.State.Day
	next, ok := sizedModel(t, NewChrome(false, true), 1, 120, 30).
		WithStore(SaveTarget{Store: store, RunID: testRunID}).OfferContinue()
	if !ok {
		t.Fatalf("OfferContinue found no run in a folder that holds one")
	}
	if next.mode != modeContinue {
		t.Fatalf("OfferContinue left mode %v, want the continue list", next.mode)
	}
	screen := frame(next)
	for _, want := range []string{"saved runs", "seed 1", "day " + itoa(wantDay),
		game.FormatMoney(before.State.Cash), store.Dir} {
		if !strings.Contains(screen, want) {
			t.Errorf("the continue list is missing %q:\n%s", want, screen)
		}
	}
	// [n] deals a fresh run and leaves the saved one alone.
	fresh := send(next, "n")
	if fresh.run.State.Day != 1 {
		t.Errorf("[n] did not deal a fresh run: day %d", fresh.run.State.Day)
	}
	if f, err := store.Load(testRunID); err != nil {
		t.Fatalf("the saved run vanished: %v", err)
	} else if f.State.Day != wantDay {
		t.Errorf("[n] disturbed the saved run: it is on day %d, want %d", f.State.Day, wantDay)
	}
	// [enter] on the list carries that run on.
	resume := send(next, "enter")
	if resume.run == nil || resume.run.State.Day != wantDay {
		t.Fatalf("carrying on did not restore day %d: %+v", wantDay, resume.run)
	}
	// A stray key on the list does nothing at all.
	if same := send(next, "b", "1"); same.run != nil {
		t.Errorf("a board key leaked into the continue list")
	}
	// A digit picks a row, out of range is refused with the list still intact.
	if picked := send(next, "1"); picked.saveSel != 0 {
		t.Errorf("[1] did not pick the first row")
	}
	if bad := send(next, "9"); !strings.Contains(statusLineOf(bad), "no run number 9") {
		t.Errorf("a digit past the end of the list is not explained: %q", statusLineOf(bad))
	}
	if _, err := store.Load(testRunID); err != nil {
		t.Errorf("browsing the list damaged the save: %v", err)
	}
}

// TestDamagedSaveIsReportedAndRecoverable is the recovery prompt: the list says
// what is wrong, and [r] opens the backup the store keeps beside it.
func TestDamagedSaveIsReportedAndRecoverable(t *testing.T) {
	store := saveStore(t)
	m, _, _ := savedAt(t, store, 4)
	// One more position, so the copy beside the file is a real earlier moment of
	// the same run rather than the file's own contents.
	m = send(m, "l", "5", "0", "0", "0", "enter")
	path, err := store.Path(testRunID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	buf, err := os.ReadFile(path + save.BakSuffix)
	if err != nil {
		t.Fatalf("the store kept no backup to recover from: %v", err)
	}
	var want save.File
	if err := json.Unmarshal(buf, &want); err != nil {
		t.Fatalf("the backup itself cannot be read: %v", err)
	}
	// Leave the first window's claim behind, the way closing a window does, so the
	// relaunch below tests the folder and not a stale marker.
	m = send(m, "q")
	if err := os.WriteFile(path, []byte(`{"state":{"day":`), 0o600); err != nil {
		t.Fatalf("damage the save: %v", err)
	}

	next, ok := sizedModel(t, NewChrome(false, true), 1, 120, 30).
		WithStore(SaveTarget{Store: store, RunID: testRunID}).OfferContinue()
	if !ok {
		t.Fatalf("a folder with a damaged save offered nothing: the player would never see it")
	}
	if next.mode != modeContinue {
		t.Fatalf("OfferContinue left mode %v", next.mode)
	}
	if !strings.Contains(frame(next), "cannot be opened") {
		t.Errorf("the damaged row does not say so:\n%s", frame(next))
	}
	// [enter] on a damaged row is refused, in the store's own words, and leaves
	// the player on the list with the other keys still available.
	blocked := send(next, "enter")
	if blocked.mode != modeContinue {
		t.Fatalf("a damaged save was accepted: mode %v", blocked.mode)
	}
	if !strings.Contains(statusLineOf(blocked), "damaged") {
		t.Errorf("the refusal does not explain the damage: %q", statusLineOf(blocked))
	}
	// [r] recovers the backup: the run as it stood before the damage, with the
	// history that produced it.
	back := send(next, "r")
	if back.run == nil {
		t.Fatalf("[r] left no run on screen: %q", statusLineOf(back))
	}
	if !reflect.DeepEqual(back.run.State, want.State) {
		t.Errorf("the recovered run is not the backup position:\n got %+v\nwant %+v", back.run.State, want.State)
	}
	if len(back.actions) != len(want.Trace.Actions) {
		t.Errorf("the recovered history is %d actions, the backup holds %d", len(back.actions), len(want.Trace.Actions))
	}
	if !strings.Contains(statusLineOf(back), save.BakSuffix) {
		t.Errorf("the recovery does not say which file is on screen: %q", statusLineOf(back))
	}
	if back.saveErr != "" {
		t.Errorf("the recovered run cannot be saved: %s", back.saveErr)
	}
	// Saving on from the backup replaces the damaged file, and it reads back clean.
	repaired := send(back, "l", "1", "0", "0", "0", "enter")
	if repaired.saveErr != "" {
		t.Fatalf("the recovered run could not be saved: %s", repaired.saveErr)
	}
	f, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("the save did not heal: %v", err)
	}
	if !reflect.DeepEqual(f.State, repaired.run.State) {
		t.Errorf("the healed save disagrees with the screen:\n file %+v\nscreen %+v", f.State, repaired.run.State)
	}
}

// TestStartingASeedOverKeepsTheOldRun covers the one deliberate backwards write:
// the player chose to replay a seed that already has a save, and the run that was
// there is kept beside it.
func TestStartingASeedOverKeepsTheOldRun(t *testing.T) {
	store := saveStore(t)
	m, _, _ := savedAt(t, store, 10)
	m = send(m, "q")
	old, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// A relaunch on the same seed: OfferContinue, then [n] for a fresh deal.
	list, ok := sizedModel(t, NewChrome(false, true), 1, 120, 30).
		WithStore(SaveTarget{Store: store, RunID: testRunID}).OfferContinue()
	if !ok {
		t.Fatalf("OfferContinue found nothing")
	}
	fresh := send(list, "n")
	if fresh.run.State.Day != 1 || fresh.run.State.Cash != 30000 {
		t.Fatalf("the new deal is not a fresh run: %+v", fresh.run.State)
	}
	fresh = send(fresh, "l", "5", "0", "0", "0", "enter")
	if fresh.saveErr != "" {
		t.Fatalf("a new run refused to save into its own slot: %s", fresh.saveErr)
	}
	now, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("Load after the new run wrote: %v", err)
	}
	if now.State.Day != 1 || len(now.Trace.Actions) != 1 {
		t.Errorf("the new run did not take over its slot: day %d, %d actions", now.State.Day, len(now.Trace.Actions))
	}
	bakPath := saveFileName(store, testRunID) + save.BakSuffix
	buf, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("the run that was replaced is not kept: %v", err)
	}
	var kept save.File
	if err := json.Unmarshal(buf, &kept); err != nil {
		t.Fatalf("the kept copy cannot be read: %v", err)
	}
	if kept.State.Day != old.State.Day || len(kept.Trace.Actions) != len(old.Trace.Actions) {
		t.Errorf("the kept copy is not the run that was there: day %d with %d actions, want day %d with %d",
			kept.State.Day, len(kept.Trace.Actions), old.State.Day, len(old.Trace.Actions))
	}
	if kept.State.Debt == now.State.Debt && kept.State.Cash == now.State.Cash {
		t.Errorf("the kept copy is the new run, not the run it replaced")
	}
}

// TestInterruptedKeyboardRunCarriesOnToTheSameEnding is the Phase 4 exit gate:
// the same seed and the same keys, split by a quit and a fresh process, end in the
// same place as one uninterrupted run of that seed.
func TestInterruptedKeyboardRunCarriesOnToTheSameEnding(t *testing.T) {
	dir := t.TempDir()
	ref, steps, err := sim.Play(sim.Prototype(1), sim.TradeRepay)
	if err != nil {
		t.Fatalf("reference run: %v", err)
	}
	if ref.State.End.Kind != game.EndClosed || ref.State.End.CashOut <= 0 {
		t.Fatalf("the reference seed no longer produces a trading victory: %+v", ref.State.End)
	}
	var acts []game.Action
	for _, st := range steps {
		if st.OK {
			acts = append(acts, st.Action)
		}
	}
	cut := len(acts) / 2
	if cut < 2 {
		t.Fatalf("the reference trace is too short to interrupt: %d actions", len(acts))
	}

	// A keyboard script is built once and split at an action boundary, so both
	// halves are the keys a player would actually press.
	type scripted struct {
		keys string
	}
	var script []scripted
	cur := content.Garage
	sel := 0
	idx := 0
	for _, st := range steps {
		if !st.OK {
			continue
		}
		if idx == cut {
			// The second half runs in a fresh process, and resume puts the
			// board cursor back at the top row: its keystrokes are written
			// from row 0, not from wherever the first process left it.
			sel = 0
		}
		script = append(script, scripted{keys: keysFor(cur, st.Action, &sel)})
		idx++
		cur = st.Market
	}

	const cols, rows = 120, 30
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pipeRead.Close()
	defer pipeWrite.Close()

	store, err := save.NewStore(dir, "evidence")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := tea.NewProgram(
		mustModel(t, NewChrome(false, true), 1).WithStore(SaveTarget{Store: store, RunID: testRunID}),
		tea.WithContext(ctx),
		tea.WithInput(pipeRead),
		tea.WithOutput(newScreen(cols, rows)),
		tea.WithWindowSize(cols, rows),
		tea.WithColorProfile(colorprofile.ASCII),
	)
	firstErr := make(chan error, 1)
	go func() { _, err := first.Run(); firstErr <- err }()

	var half strings.Builder
	for _, s := range script[:cut] {
		half.WriteString(s.keys)
	}
	if _, err := pipeWrite.Write([]byte(half.String())); err != nil {
		t.Fatalf("send the first half: %v", err)
	}
	if _, err := pipeWrite.Write([]byte("q")); err != nil {
		t.Fatalf("send q: %v", err)
	}
	if err := <-firstErr; err != nil {
		t.Fatalf("the interrupted run ended with an error: %v", err)
	}

	// The file on disk is exactly the reference position at the cut, because the
	// save carries its own history and a replay of that history is what a resume
	// trusts.
	saved, err := store.Load(testRunID)
	if err != nil {
		t.Fatalf("Load after the quit: %v", err)
	}
	part, _, err := game.Trace{Config: ref.Config(), Actions: acts[:cut]}.Replay()
	if err != nil {
		t.Fatalf("replay the first half: %v", err)
	}
	if !reflect.DeepEqual(saved.State, part.State) {
		t.Errorf("the save is not the position the reference run reached at the cut:\n got %+v\nwant %+v", saved.State, part.State)
	}

	// A new process on the same folder and the same seed: the continue list, one
	// Enter, and the rest of the same keys.
	pipeRead2, pipeWrite2, err := os.Pipe()
	if err != nil {
		t.Fatalf("second pipe: %v", err)
	}
	defer pipeRead2.Close()
	defer pipeWrite2.Close()
	scr2 := newScreen(cols, rows)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second, err := save.NewStore(dir, "evidence")
	if err != nil {
		t.Fatalf("reopen the store: %v", err)
	}
	offered, ok := mustModel(t, NewChrome(false, true), 1).
		WithStore(SaveTarget{Store: second, RunID: testRunID}).OfferContinue()
	if !ok {
		t.Fatalf("a relaunch found no run to carry on")
	}
	prog2 := tea.NewProgram(offered,
		tea.WithContext(ctx2),
		tea.WithInput(pipeRead2),
		tea.WithOutput(scr2),
		tea.WithWindowSize(cols, rows),
		tea.WithColorProfile(colorprofile.ASCII),
	)
	go func() { _, err := prog2.Run(); firstErr <- err }()

	wait := func(what string, pred func(string) bool) string {
		t.Helper()
		if !scr2.waitUntil(20*time.Second, pred) {
			t.Fatalf("%s\nscreen was:\n%s", what, scr2.text())
		}
		return scr2.text()
	}
	listed := wait("the continue list never painted", func(s string) bool {
		return strings.Contains(s, "saved runs") && strings.Contains(s, "seed 1")
	})
	if !strings.Contains(listed, "day "+itoa(part.State.Day)) {
		t.Errorf("the continue list does not show the day the run stopped on (%d):\n%s", part.State.Day, listed)
	}
	if _, err := pipeWrite2.Write([]byte("\r")); err != nil {
		t.Fatalf("send enter: %v", err)
	}
	wait("the resumed board never painted", func(s string) bool {
		return strings.Contains(s, "PRODUCT") && strings.Contains(s, "Day "+itoa(part.State.Day)+"/30")
	})
	var rest strings.Builder
	for _, s := range script[cut:] {
		rest.WriteString(s.keys)
	}
	if _, err := pipeWrite2.Write([]byte(rest.String())); err != nil {
		t.Fatalf("send the rest: %v", err)
	}
	final := wait("the carried-on run never reached an ending", func(s string) bool {
		return strings.Contains(s, "CASH OUT") || strings.Contains(s, "DEFICIENCY")
	})
	if want := "CASH OUT: " + game.FormatMoney(ref.State.End.CashOut); !strings.Contains(strings.Join(strings.Fields(final), " "), want) {
		t.Errorf("the carried-on run closed at a different number than the uninterrupted run: want %q\nscreen:\n%s", want, final)
	}
	stt := ref.State.Stats
	for _, want := range []string{
		"borrowed " + game.FormatMoney(stt.Borrowed),
		"repaid " + game.FormatMoney(stt.Repaid),
		"interest paid " + game.FormatMoney(stt.InterestPaid),
		"forced sales " + itoa(stt.ForcedUnits),
	} {
		if !strings.Contains(strings.Join(strings.Fields(final), " "), want) {
			t.Errorf("the carried-on run's receipts disagree with the uninterrupted run, missing %q:\n%s", want, final)
		}
	}
	if _, err := pipeWrite2.Write([]byte("\x03")); err != nil {
		t.Fatalf("send ctrl+c: %v", err)
	}
	if err := <-firstErr; err != nil {
		t.Fatalf("ctrl+c ended the carried-on run with an error: %v", err)
	}
}

func TestStartupPersistsInitialRunAndOffersContinue(t *testing.T) {
	store := saveStore(t)
	m := sizedModel(t, NewChrome(false, true), 1, 120, 30).StartPersistence(SaveTarget{Store: store, RunID: testRunID})
	if m.saveErr != "" {
		t.Fatal(m.saveErr)
	}
	f, err := store.Load(testRunID)
	if err != nil {
		t.Fatal(err)
	}
	if f.State.Day != 1 || len(f.Trace.Actions) != 0 {
		t.Fatal("initial state not saved")
	}
	m.Close()
	next := sizedModel(t, NewChrome(false, true), 1, 120, 30).StartPersistence(SaveTarget{Store: store, RunID: testRunID})
	if next.mode != modeContinue || next.lock != nil {
		t.Fatal("startup must offer Continue without claiming a run")
	}
	next = send(next, "enter")
	if next.mode != modeBoard || next.lock == nil {
		t.Fatal("Continue did not acquire the run")
	}
	next.Close()
}
