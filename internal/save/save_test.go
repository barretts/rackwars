package save

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"rackwars/internal/content"
	"rackwars/internal/game"
	"rackwars/internal/sim"
)

// sampleActions is a short but real morning: freight, a draw against the note,
// a purchase, a day close, a partial repayment, another close. Everything the
// save format has to carry shows up somewhere in it.
func sampleActions() []game.Action {
	return []game.Action{
		{Kind: game.ActTravel, Market: content.Ashburn},
		{Kind: game.ActBorrow, Amount: 5000},
		{Kind: game.ActBuy, Product: content.Card4090, Qty: 2},
		{Kind: game.ActStay},
		{Kind: game.ActRepay, Amount: 2000},
		{Kind: game.ActStay},
	}
}

// playRun deals seed 1 and applies the actions, returning the run and the list
// of actions it actually accepted (the save format only ever records accepted
// ones).
func playRun(t *testing.T, actions []game.Action) (*game.Run, []game.Action) {
	t.Helper()
	r, err := game.New(game.DefaultPrototypeConfig(1))
	if err != nil {
		t.Fatalf("new run: %v", err)
	}
	accepted := make([]game.Action, 0, len(actions))
	for i, a := range actions {
		if r.State.Over != game.EndOpen {
			break
		}
		if _, res := r.Apply(a); !res.OK {
			t.Fatalf("setup action %d (%v) refused: %s", i, a, res.Reason)
		}
		accepted = append(accepted, a)
	}
	return r, accepted
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir(), "test-build")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return s
}

// saveRun writes a save for the run and returns the store, the file, and the
// lock, releasing the lock when the test ends.
func saveRun(t *testing.T, s *Store, id string, run *game.Run, actions []game.Action) File {
	t.Helper()
	lock, err := s.Hold(id)
	if err != nil {
		t.Fatalf("hold %s: %v", id, err)
	}
	defer func() { _ = lock.Release() }()
	f := Of(run, actions, s.Engine)
	if err := s.Save(id, f, lock); err != nil {
		t.Fatalf("save %s: %v", id, err)
	}
	return f
}

// loadErr and holdErr reduce a two-value call to the error a test is checking.
func mustPath(t *testing.T, s *Store, id string) string {
	t.Helper()
	p, err := s.Path(id)
	if err != nil {
		t.Fatalf("path for run id %q: %v", id, err)
	}
	return p
}

func loadErr(s *Store, id string) error {
	_, err := s.Load(id)
	return err
}

func holdErr(s *Store, id string) error {
	_, err := s.Hold(id)
	return err
}

func kindOf(t *testing.T, err error, want string) *Failure {
	t.Helper()
	if err == nil {
		t.Fatalf("wanted a %s failure, got none", want)
	}
	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("wanted a save failure (%s), got %v", want, err)
	}
	if f.Kind != want {
		t.Fatalf("wanted failure kind %q, got %q (%v)", want, f.Kind, err)
	}
	return f
}

func TestSaveThenLoadRestoresTheRunExactly(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	want := saveRun(t, s, "seed-1", run, accepted)

	// A different Store over the same directory is a different process as far as
	// the file is concerned.
	other, err := NewStore(s.Dir, "other-build")
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	got, err := other.Load("seed-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got.State, want.State) {
		t.Fatalf("the loaded state is not the saved one:\n got %+v\nwant %+v", got.State, want.State)
	}
	if !reflect.DeepEqual(got.Trace.Actions, accepted) {
		t.Fatalf("the loaded action history is %v, want %v", got.Trace.Actions, accepted)
	}
	if got.Trace.Config.Seed != run.Config().Seed {
		t.Fatalf("the loaded config deals from seed %d, want %d", got.Trace.Config.Seed, run.Config().Seed)
	}
	if got.Engine != "test-build" {
		t.Fatalf("the file records engine %q, want the build that wrote it", got.Engine)
	}
	resumed, err := got.Run()
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.State.Cash != want.State.Cash || resumed.State.Debt != want.State.Debt {
		t.Fatalf("the resumed run holds %s cash and %s debt, want %s and %s",
			game.FormatMoney(resumed.State.Cash), game.FormatMoney(resumed.State.Debt),
			game.FormatMoney(want.State.Cash), game.FormatMoney(want.State.Debt))
	}
}

// TestSavedFileCarriesTheFactsAPlayerNeeds pins the parts of the file a player
// or a bug report has to be able to read: identity, the visible state, and the
// action history. These are load-bearing field names, so the test fails when
// they are renamed rather than after a build ships.
func TestSavedFileCarriesTheFactsAPlayerNeeds(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	saveRun(t, s, "seed-1", run, accepted)

	raw, err := os.ReadFile(mustPath(t, s, "seed-1"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the save is not readable JSON: %v", err)
	}
	for _, key := range []string{"engineVersion", "written", "trace", "state"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("the save carries no %q", key)
		}
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(doc["state"], &state); err != nil {
		t.Fatalf("the state is not an object: %v", err)
	}
	for _, key := range []string{"day", "market", "cash", "debt", "owned", "costBasis", "stock",
		"quotes", "seenDay", "over", "end", "stats"} {
		if _, ok := state[key]; !ok {
			t.Errorf("the saved state carries no %q", key)
		}
	}
	var trace map[string]json.RawMessage
	if err := json.Unmarshal(doc["trace"], &trace); err != nil {
		t.Fatalf("the trace is not an object: %v", err)
	}
	for _, key := range []string{"config", "actions"} {
		if _, ok := trace[key]; !ok {
			t.Errorf("the trace carries no %q", key)
		}
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(trace["config"], &config); err != nil {
		t.Fatalf("the recorded config is not an object: %v", err)
	}
	for _, key := range []string{"seed", "products", "markets", "rules"} {
		if _, ok := config[key]; !ok {
			t.Errorf("the recorded config names no %q", key)
		}
	}
}

// TestInitialRunIsSavable proves a run can be put down before the player has
// done anything, which the plan requires.
func TestInitialRunIsSavable(t *testing.T) {
	s := testStore(t)
	run, err := game.New(game.DefaultPrototypeConfig(7))
	if err != nil {
		t.Fatalf("new run: %v", err)
	}
	saveRun(t, s, "fresh", run, nil)
	got, err := s.Load("fresh")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.State.Day != 1 || got.State.Market != content.Garage {
		t.Fatalf("the fresh save is day %d in %s, want day 1 at the garage", got.State.Day, got.State.Market)
	}
	if got.State.Cash != 10000 || got.State.Debt != 20000 {
		t.Fatalf("the fresh save holds %s cash and %s debt, want $10,000 and $20,000",
			game.FormatMoney(got.State.Cash), game.FormatMoney(got.State.Debt))
	}
	if len(got.Trace.Actions) != 0 {
		t.Fatalf("a run that did nothing recorded %d actions", len(got.Trace.Actions))
	}
	if _, err := got.Run(); err != nil {
		t.Fatalf("resuming a fresh run: %v", err)
	}
	entries, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 || entries[0].Day != 1 {
		t.Fatalf("the store lists %+v, want one run on day 1", entries)
	}
}

func TestListReportsADamagedSaveInsteadOfHidingIt(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	saveRun(t, s, "good", run, accepted)
	if err := os.WriteFile(mustPath(t, s, "broken"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("plant damaged save: %v", err)
	}
	entries, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("the store lists %d runs, want 2: %+v", len(entries), entries)
	}
	var good, broken *Entry
	for i := range entries {
		switch entries[i].ID {
		case "good":
			good = &entries[i]
		case "broken":
			broken = &entries[i]
		}
	}
	if good == nil || good.Err != nil || good.Day == 0 || good.Market == "" {
		t.Fatalf("the readable run is reported as %+v", good)
	}
	if broken == nil || broken.Err == nil {
		t.Fatalf("the damaged run is reported as %+v", broken)
	}
	if !strings.Contains(broken.Err.Error(), "damaged") {
		t.Fatalf("the damaged run does not say it is damaged: %v", broken.Err)
	}
}

func TestSaveDirectoryChoices(t *testing.T) {
	want := t.TempDir()
	dir, err := Dir(want)
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if dir != filepath.Clean(want) {
		t.Fatalf("an override must be used as given: got %q, want %q", dir, filepath.Clean(want))
	}
	nested, err := Dir(filepath.Join(want, "deeper.level", "s-aves"))
	if err != nil {
		t.Fatalf("nested override: %v", err)
	}
	if nested != filepath.Join(filepath.Clean(want), "deeper.level", "s-aves") {
		t.Fatalf("a nested override came out as %q", nested)
	}
	def, err := Dir("")
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if !strings.HasSuffix(def, filepath.Join("rackwars", "saves")) {
		t.Fatalf("the default save directory is %q; it must live under the per-user config directory", def)
	}
	if _, err := Dir("\x00"); err == nil {
		t.Fatalf("an unusable configuration directory was accepted")
	}

	deep := filepath.Join(t.TempDir(), "deep", "deeper")
	s, err := NewStore(deep, "test-build")
	if err != nil {
		t.Fatalf("a nested save directory must be created, got %v", err)
	}
	if s.Dir != filepath.Clean(deep) {
		t.Fatalf("the store kept %q, want %q", s.Dir, filepath.Clean(deep))
	}
	if fi, err := os.Stat(s.Dir); err != nil || !fi.IsDir() {
		t.Fatalf("the save directory does not exist: %v", err)
	}
	// A directory that cannot be created fails here, at start-up, rather than on
	// the first autosave: the parent exists as a file.
	blocker := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("plant blocker: %v", err)
	}
	if _, err := NewStore(filepath.Join(blocker, "saves"), "test-build"); err == nil {
		t.Fatalf("an unusable save directory was accepted at start-up")
	}
}

func TestRunIDsThatCouldReachOutsideTheSaveDirectoryAreRefused(t *testing.T) {
	s := testStore(t)
	for _, id := range []string{"", "../evil", "a/b", "..", ".", "with space", "trailing.", strings.Repeat("x", 65)} {
		if _, err := s.Path(id); err == nil {
			t.Errorf("the run id %q was accepted as a file name", id)
		}
		if err := s.Save(id, File{}, nil); err == nil {
			t.Errorf("the run id %q was accepted for writing", id)
		}
	}
	entries, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("refused ids left files behind: %+v", entries)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.Dir), "evil")); !os.IsNotExist(err) {
		t.Fatalf("a run id escaped the save directory")
	}
}

func TestTraceExportReplaysToTheSameEnding(t *testing.T) {
	run, steps, err := sim.Play(sim.Prototype(1), sim.TradeRepay)
	if err != nil {
		t.Fatalf("scripted run: %v", err)
	}
	accepted := actionsOf(steps)
	if run.State.Over == game.EndOpen {
		t.Fatalf("the scripted run did not reach an ending; the test needs one")
	}
	s := testStore(t)
	saveRun(t, s, "exportable", run, accepted)

	jsonPath, txtPath, err := s.Export("exportable", game.Trace{Config: run.Config(), Actions: accepted})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	for _, p := range []string{jsonPath, txtPath} {
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			t.Fatalf("the exported file %s is missing or empty: %v", p, err)
		}
	}
	tr, err := s.LoadTrace(jsonPath)
	if err != nil {
		t.Fatalf("re-load the exported trace: %v", err)
	}
	replayRun, steps, err := tr.Replay()
	if err != nil {
		t.Fatalf("replay the exported trace: %v", err)
	}
	if len(steps) != len(accepted) {
		t.Fatalf("the replay ran %d steps, want %d", len(steps), len(accepted))
	}
	if replayRun.State.End != run.State.End {
		t.Fatalf("the replay ends %v, the saved run ends %v", replayRun.State.End, run.State.End)
	}
	text, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("read the exported log: %v", err)
	}
	if !strings.Contains(string(text), game.FormatMoney(run.State.End.CashOut)) &&
		!strings.Contains(string(text), game.FormatMoney(run.State.Debt)) {
		t.Fatalf("the exported log does not carry the run's money:\n%s", text)
	}
}

func TestLockingBlocksASecondWindowAndTakesOverAStaleOne(t *testing.T) {
	s := testStore(t)
	first, err := s.Hold("duplex")
	if err != nil {
		t.Fatalf("first hold: %v", err)
	}
	if err := first.Touch(); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	second := kindOf(t, holdErr(s, "duplex"), "busy")
	if !strings.Contains(second.Message, "already open by") {
		t.Fatalf("the busy message does not say the run is open elsewhere: %s", second.Message)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	third, err := s.Hold("duplex")
	if err != nil {
		t.Fatalf("a released run must be openable again: %v", err)
	}
	_ = third.Release()

	// A marker left by a killed process must not lock the player out of their own
	// save forever.
	stale := mustPath(t, s, "duplex") + LockSuffix
	old := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(stale, []byte("pid 99999 on long-gone-machine\n"+old+"\n"), 0o600); err != nil {
		t.Fatalf("plant stale marker: %v", err)
	}
	fourth := &Lock{Path: stale}
	if err := fourth.Touch(); err != nil {
		t.Fatalf("re-stamp the fresh marker: %v", err)
	}
	fifth, err := s.Hold("duplex")
	if err != nil {
		t.Fatalf("a stale marker must be taken over: %v", err)
	}
	if !strings.Contains(fifth.Note, "took over") {
		t.Fatalf("taking over a stale marker was not reported to the player: %+v", fifth)
	}
	_ = fifth.Release()
}

func TestUnsupportedVersionsAreRefused(t *testing.T) {
	r, actions := playRun(t, nil)
	original := Of(r, actions, "test")
	for _, field := range []string{"schema", "engine", "content"} {
		t.Run(field, func(t *testing.T) {
			f := original
			switch field {
			case "schema":
				f.Schema++
			case "engine":
				f.RulesVersion = "future"
			case "content":
				f.Content = "future"
			}
			s := testStore(t)
			path, _ := s.Path("version")
			b, _ := json.Marshal(f)
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load("version"); err == nil {
				t.Fatal("unsupported version was loaded")
			}
		})
	}
}
