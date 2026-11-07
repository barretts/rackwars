package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rackwars/internal/content"
	"rackwars/internal/game"
	"rackwars/internal/sim"
)

// actionsOf lifts the accepted actions out of a replay so a save can carry them.
func actionsOf(steps []game.Step) []game.Action {
	out := make([]game.Action, 0, len(steps))
	for _, st := range steps {
		if st.OK {
			out = append(out, st.Action)
		}
	}
	return out
}

// TestInterruptedRunFinishesExactlyLikeAnUninterruptedOne is the Phase 4 gate in
// one test: the scripted 30-day runs are saved and re-loaded every five actions,
// and the interrupted run must end with exactly the same ending and statistics as
// the run that was never interrupted. Anything that rerolls a morning, re-draws a
// shock, or re-accrues interest on resume fails here, on both the win and the
// repossession.
func TestInterruptedRunFinishesExactlyLikeAnUninterruptedOne(t *testing.T) {
	for _, policy := range []sim.Policy{sim.TradeRepay, sim.Leveraged} {
		whole, steps, err := sim.Play(sim.Prototype(1), policy)
		if err != nil {
			t.Fatalf("uninterrupted run: %v", err)
		}
		if whole.State.Over == game.EndOpen {
			t.Fatalf("the scripted run never ended")
		}
		history := actionsOf(steps)

		s := testStore(t)
		r, err := game.New(game.DefaultPrototypeConfig(1))
		if err != nil {
			t.Fatalf("interrupted run: %v", err)
		}
		for i, a := range history {
			if _, res := r.Apply(a); !res.OK {
				t.Fatalf("policy %v action %d (%v) was refused on the interrupted path: %s",
					policy, i+1, a, res.Reason)
			}
			if (i+1)%5 != 0 && i != len(history)-1 {
				continue
			}
			lock, err := s.Hold("interrupted")
			if err != nil {
				t.Fatalf("hold: %v", err)
			}
			f := Of(r, history[:i+1], "test-build")
			if err := s.Save("interrupted", f, lock); err != nil {
				t.Fatalf("save after action %d: %v", i+1, err)
			}
			if err := lock.Release(); err != nil {
				t.Fatalf("release: %v", err)
			}
			got, err := s.Load("interrupted")
			if err != nil {
				t.Fatalf("load after action %d: %v", i+1, err)
			}
			if !reflect.DeepEqual(got.State, r.State) {
				t.Fatalf("policy %v: the reload after action %d is not the board that was saved\n got %+v\nwant %+v",
					policy, i+1, got.State, r.State)
			}
			resumed, err := got.Run()
			if err != nil {
				t.Fatalf("resume after action %d: %v", i+1, err)
			}
			r = resumed
		}
		if r.State.End != whole.State.End {
			t.Fatalf("policy %v: the interrupted run ends %+v, the uninterrupted run ends %+v",
				policy, r.State.End, whole.State.End)
		}
		if r.State.Stats != whole.State.Stats {
			t.Fatalf("policy %v: the interrupted run reports %+v, the uninterrupted run reports %+v",
				policy, r.State.Stats, whole.State.Stats)
		}
	}
}

// TestReloadDoesNotTouchTheMoneyOrTheMarket checks the two things a naive
// resume gets wrong: charging the day's interest again, and dealing the morning
// prices a second time.
func TestReloadDoesNotTouchTheMoneyOrTheMarket(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	before := saveRun(t, s, "quiet", run, accepted)

	var last File
	for i := 0; i < 3; i++ {
		got, err := s.Load("quiet")
		if err != nil {
			t.Fatalf("load %d: %v", i+1, err)
		}
		if got.State.Debt != before.State.Debt {
			t.Fatalf("load %d changed the debt from %s to %s",
				i+1, game.FormatMoney(before.State.Debt), game.FormatMoney(got.State.Debt))
		}
		if got.State.Cash != before.State.Cash {
			t.Fatalf("load %d changed the cash from %s to %s",
				i+1, game.FormatMoney(before.State.Cash), game.FormatMoney(got.State.Cash))
		}
		if !reflect.DeepEqual(got.State.Quotes, before.State.Quotes) {
			t.Fatalf("load %d re-dealt the prices the player already saw", i+1)
		}
		if !reflect.DeepEqual(got.State.Stock, before.State.Stock) {
			t.Fatalf("load %d re-dealt the market's supply", i+1)
		}
		last = got
	}
	if !reflect.DeepEqual(last.State, before.State) {
		t.Fatalf("the file changed under repeated reads:\n got %+v\nwant %+v", last.State, before.State)
	}
}

func TestGarbageSaveIsRefusedWithAWayOut(t *testing.T) {
	s := testStore(t)
	for _, body := range []string{"", "{", "not json at all", `{"trace":`, `{"state":{"day":}}`} {
		name := "garbage"
		if err := os.WriteFile(mustPath(t, s, name), []byte(body), 0o600); err != nil {
			t.Fatalf("plant garbage: %v", err)
		}
		_, err := s.Load(name)
		f := kindOf(t, err, "corrupt")
		if !strings.Contains(f.Message, "Recover") {
			t.Fatalf("a corrupt save does not tell the player how to recover: %s", f.Message)
		}
	}
}

// TestHandEditedCashIsCaughtByItsOwnHistory is the invariant the plan asks for:
// the recorded actions must replay to the state in the file. A save whose cash
// does not match its own history is refused with the field named, not played.
func TestHandEditedCashIsCaughtByItsOwnHistory(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	f := saveRun(t, s, "edited", run, accepted)

	f.State.Cash += 500000
	buf, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if err := os.WriteFile(mustPath(t, s, "edited"), buf, 0o600); err != nil {
		t.Fatalf("plant edited save: %v", err)
	}
	_, err = s.Load("edited")
	got := kindOf(t, err, "drift")
	if !strings.Contains(got.Message, "cash") || !strings.Contains(got.Message, "Recover") {
		t.Fatalf("the edited save was refused without naming the field and the remedy: %s", got.Message)
	}
}

// TestSaveThatNamesAnUnknownTownIsRefused covers the case a version string was
// meant to cover: a save that does not belong to this build's content.
func TestSaveThatNamesAnUnknownTownIsRefused(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())

	// Writing it is refused too, so a bad state never reaches the disk.
	lock, err := s.Hold("alien")
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	defer func() { _ = lock.Release() }()
	bad := Of(run, accepted, s.Engine)
	bad.State.Market = content.TheDalles
	f := kindOf(t, s.Save("alien", bad, lock), "internal")
	if _, err := os.Stat(mustPath(t, s, "alien")); !os.IsNotExist(err) {
		t.Fatalf("a refused write left a file behind: %s", f.Path)
	}

	// A file written by another build names a town this build does not have; the
	// refusal says which town and which cast.
	other := Of(run, accepted, "some-other-build")
	other.State.Market = content.TheDalles
	buf, err := json.MarshalIndent(other, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(mustPath(t, s, "alien"), buf, 0o600); err != nil {
		t.Fatalf("plant foreign save: %v", err)
	}
	_, err = s.Load("alien")
	got := kindOf(t, err, "corrupt")
	if !strings.Contains(got.Message, string(content.TheDalles)) {
		t.Fatalf("the refusal does not name the town the save asks for: %s", got.Message)
	}
}

// TestPendingChoiceIsNeverSkipped keeps the plan's requirement that a save
// waiting on a choice cannot be resumed past it. The one pending kind this
// build can show is an encounter; a save naming any other waits on something
// this build cannot render.
func TestPendingChoiceIsNeverSkipped(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	f := Of(run, accepted, s.Engine)
	f.Pending = "timecapsule:the-fire"
	buf, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(mustPath(t, s, "waiting"), buf, 0o600); err != nil {
		t.Fatalf("plant save with a pending choice: %v", err)
	}
	_, err = s.Load("waiting")
	got := kindOf(t, err, "pending")
	if !strings.Contains(got.Message, "the-fire") || !strings.Contains(got.Message, "cannot show") {
		t.Fatalf("the refusal does not name the choice it cannot show: %s", got.Message)
	}
}

func TestRecoveryFindsTheLastGoodCopy(t *testing.T) {
	s := testStore(t)
	day2, actions2 := playRun(t, sampleActions()[:3])
	f2 := saveRun(t, s, "rescued", day2, actions2)

	run3, actions3 := playRun(t, sampleActions())
	f3 := saveRun(t, s, "rescued", run3, actions3)
	if f3.State.Day <= f2.State.Day {
		t.Fatalf("the second save did not advance the run: day %d then day %d", f2.State.Day, f3.State.Day)
	}

	// The live file is damaged; the backup from the previous autosave is intact.
	if err := os.WriteFile(mustPath(t, s, "rescued"), []byte("{ truncated"), 0o600); err != nil {
		t.Fatalf("damage the live save: %v", err)
	}
	got, from, err := s.Recover("rescued")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if !strings.HasSuffix(from, BakSuffix) {
		t.Fatalf("recovery reported %q, expected the backup file", from)
	}
	if got.State.Day != f2.State.Day || got.State.Cash != f2.State.Cash {
		t.Fatalf("recovery returned day %d with %s, expected day %d with %s",
			got.State.Day, game.FormatMoney(got.State.Cash),
			f2.State.Day, game.FormatMoney(f2.State.Cash))
	}
	if _, err := got.Run(); err != nil {
		t.Fatalf("the recovered run will not resume: %v", err)
	}

	// Nothing good left to fall back on: the player is told both places it looked.
	if err := os.WriteFile(mustPath(t, s, "rescued")+BakSuffix, []byte("{ truncated"), 0o600); err != nil {
		t.Fatalf("damage the backup: %v", err)
	}
	f := kindOf(t, func() error { _, _, err := s.Recover("rescued"); return err }(), "no-backup")
	for _, p := range []string{mustPath(t, s, "rescued"), mustPath(t, s, "rescued") + BakSuffix} {
		if !strings.Contains(f.Message, p) {
			t.Fatalf("the recovery failure does not name %s: %s", p, f.Message)
		}
	}
}

// TestFailedWriteKeepsTheRunAndSaysHowToRetry is the required "save failed"
// behaviour: the run in memory is untouched, the previous file on disk is
// byte-for-byte what it was, and the message says what to do.
func TestFailedWriteKeepsTheRunAndSaysHowToRetry(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	lock, err := s.Hold("stubborn")
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	defer func() { _ = lock.Release() }()
	f := Of(run, accepted, s.Engine)
	if err := s.Save("stubborn", f, lock); err != nil {
		t.Fatalf("first save: %v", err)
	}
	path := mustPath(t, s, "stubborn")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the first save: %v", err)
	}

	// Make the save directory unwritable by replacing its parent with a file.
	blocked := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("plant blocker: %v", err)
	}
	s.Dir = filepath.Join(blocked, "saves")

	fail := kindOf(t, s.Save("stubborn", f, lock), "write")
	if !strings.Contains(fail.Message, "still in memory") || !strings.Contains(fail.Message, "--save-dir") {
		t.Fatalf("the save failure does not tell the player what to do: %s", fail.Message)
	}
	if !strings.Contains(fail.Message, "to try again") {
		t.Fatalf("the save failure offers no retry: %s", fail.Message)
	}

	// The run in memory is exactly what it was, and the old file is untouched.
	if run.State.Cash != f.State.Cash || run.State.Debt != f.State.Debt {
		t.Fatalf("a failed save changed the run in memory")
	}
	s.Dir = filepath.Dir(path)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read the first save: %v", err)
	}
	if string(after) != string(good) {
		t.Fatalf("a failed save changed the file on disk")
	}
	if tmp, err := filepath.Glob(filepath.Join(s.Dir, "*.tmp*")); err != nil || len(tmp) != 0 {
		t.Fatalf("a failed save left temporary files behind: %v", tmp)
	}
}

// TestOutsideWriteIsNeverOverwritten protects two instances of the game sharing
// one save directory, which is exactly what a --save-dir mistake produces.
func TestOutsideWriteIsNeverOverwritten(t *testing.T) {
	s := testStore(t)
	run, accepted := playRun(t, sampleActions())
	lock, err := s.Hold("shared")
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	defer func() { _ = lock.Release() }()
	f := Of(run, accepted, s.Engine)
	if err := s.Save("shared", f, lock); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Somebody else wrote this run: a second window, or an old copy restored over
	// the top of it.
	other := Of(run, accepted, "second-window")
	other.State.Cash += 1
	buf, err := json.MarshalIndent(other, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(mustPath(t, s, "shared"), buf, 0o600); err != nil {
		t.Fatalf("plant foreign write: %v", err)
	}

	fail := kindOf(t, s.Save("shared", f, lock), "clobbered")
	if !strings.Contains(fail.Message, "changed since") || !strings.Contains(fail.Message, "nothing was overwritten") {
		t.Fatalf("the clobber guard does not explain the collision: %s", fail.Message)
	}
	now, err := os.ReadFile(mustPath(t, s, "shared"))
	if err != nil {
		t.Fatalf("re-read the foreign save: %v", err)
	}
	if string(now) != string(buf) {
		t.Fatalf("the foreign write was overwritten anyway")
	}
}
