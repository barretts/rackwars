package tui

import (
	"fmt"

	"rackwars/internal/game"
	"rackwars/internal/save"

	tea "charm.land/bubbletea/v2"
)

// SaveTarget is what a model needs in order to persist a run: an open store and
// the id this run is filed under. A model built without one - the Phase 3 tests,
// or a run started with saving turned off - never writes, which keeps the whole
// save path out of the rendering path.
type SaveTarget struct {
	Store *save.Store
	RunID string
}

// WithStore attaches the save target and takes this instance's claim on the run,
// so a second window opening the same folder is told about it rather than
// silently overwriting it.
func (m Model) WithStore(t SaveTarget) Model {
	m.store, m.runID = t.Store, t.RunID
	if t.Store == nil {
		return m
	}
	lock, err := t.Store.Hold(t.RunID)
	if err != nil {
		m.saveErr = "Save failed: " + err.Error()
		m.status, m.statusBad = m.saveErr, true
		return m
	}
	m.lock = lock
	return m
}

// recordAction appends an action the engine accepted to the run's history. Only
// accepted actions are recorded, so the trace on disk is exactly the sequence
// that produced the saved state and a replay of it cannot disagree with the save.
func (m Model) recordAction(a game.Action) Model {
	if m.run == nil {
		return m
	}
	m.actions = append(m.actions, a)
	return m
}

// writeSave is the autosave. It runs only after the engine has accepted and
// applied a complete state change, so a saved file is always a position the
// player was really standing in: never a half-finished transaction, never a
// refusal.
//
// A failure is never silent and never fatal. The run stays exactly as it was in
// memory, the status line says what went wrong and how to retry, and the panel
// keeps telling the player the board on screen is not on disk. Nothing on this
// path can change the game.
func (m Model) writeSave() Model {
	if m.store == nil || m.run == nil {
		return m
	}
	if m.lock == nil {
		held, err := m.store.Hold(m.runID)
		if err != nil {
			return m.saveFailed(err)
		}
		m.lock = held
	}
	f := save.Of(m.run, m.actions, m.store.Engine)
	write := m.store.Save
	if m.restart {
		// The player chose to start this seed over, so a write that moves the slot
		// back to day 1 is what was asked for; the store still keeps the run it is
		// replacing in a .bak copy.
		write = m.store.Restart
	}
	if err := write(m.runID, f, m.lock); err != nil {
		return m.saveFailed(err)
	}
	m.restart = false
	m.saveErr = ""
	m.savedTo = saveFileName(m.store, m.runID)
	return m
}

// saveFileName resolves where a run's save lives. It runs on state changes, never
// from the render path, and falls back to the folder itself when the run id is
// unusable, so a message always names something the player can look at.
func saveFileName(s *save.Store, id string) string {
	path, err := s.Path(id)
	if err != nil {
		return s.Dir
	}
	return path
}

func (m Model) saveFailed(err error) Model {
	m.saveErr = "Save failed: " + err.Error()
	m.status, m.statusBad = m.saveErr, true
	return m
}

// saveAndQuit is the [S] key: put the run down and leave. The run is only ended
// once the write has succeeded, so a failed save never costs a run; the player
// sees the failure and can retry, or leave with [q] and lose only the autosave.
func (m Model) saveAndQuit() (tea.Model, tea.Cmd) {
	if m.store == nil || m.run == nil {
		m.status = "Nothing to save: this run is not being written to a folder."
		m.statusBad = true
		return m, nil
	}
	m = m.writeSave()
	if m.saveErr != "" {
		return m, nil
	}
	m.releaseSave()
	m.quitNote = "Run saved to " + m.savedTo + ". Start the same seed, or pick it from the continue list, to carry on."
	return m, tea.Quit
}

// exportTrace writes the run's action trace in both forms: the machine-replayable
// JSON a bug report attaches, and the day-by-day log a person can read. This is
// the same history the save carries, so both describe the run on screen.
func (m Model) exportTrace() Model {
	if m.store == nil || m.run == nil {
		m.status = "Nothing to export: this run is not being written to a folder."
		m.statusBad = true
		return m
	}
	jsonPath, textPath, err := m.store.Export(m.runID, game.Trace{Config: m.run.Config(), Actions: m.actions})
	if err != nil {
		return m.saveFailed(err)
	}
	m.status = "Trace written: " + jsonPath + " and " + textPath
	m.statusBad = false
	return m
}

// OfferContinue moves the program to the continue list when the save folder holds
// anything at all. A damaged save is listed too, with its problem: this list is
// where the player learns a file is hurt, and [r] on that row is the only road to
// the backup copy sitting next to it.
func (m Model) OfferContinue() (Model, bool) {
	if m.store == nil {
		return m, false
	}
	entries, err := m.store.List()
	if err != nil {
		m.status, m.statusBad = "The save folder could not be read: "+err.Error(), true
		return m, false
	}
	if len(entries) == 0 {
		return m, false
	}
	m.releaseSave()
	m.run = nil
	m.saves, m.saveSel, m.mode = entries, 0, modeContinue
	return m, true
}

// continueRun loads the highlighted save and resumes it. A file that refuses to
// load stays refused: the store's own sentence is shown, and the player is left
// on the list with the other runs and the new-run key still available.
func (m Model) continueRun() (Model, error) {
	if m.store == nil || m.saveSel >= len(m.saves) {
		return m, fmt.Errorf("no saved run is selected")
	}
	id := m.saves[m.saveSel].ID
	f, err := m.store.Load(id)
	if err != nil {
		return m, err
	}
	run, err := f.Run()
	if err != nil {
		return m, err
	}
	lock, err := m.store.Hold(id)
	if err != nil {
		return m, err
	}
	m.run = run
	m.seed = run.Config().Seed
	m.actions = append([]game.Action(nil), f.Trace.Actions...)
	m.runID = id
	m.lock = lock
	m.mode = modeBoard
	m.sel = 0
	m.saves = nil
	m.dayLog = nil
	m.status = "Carrying on: seed " + itoa(int(m.seed)) + ", day " + itoa(run.State.Day) + "."
	m.statusBad = false
	m.saveErr = ""
	return m, nil
}

// recoverRun is the [r] key on the continue list: open the .bak copy of the
// highlighted run, which is the last version the game itself wrote that survived.
// It names the file it came from, because the run on screen is now a different
// file than the list row suggests.
func (m Model) recoverRun() (Model, error) {
	if m.store == nil || m.saveSel >= len(m.saves) {
		return m, save.NotFound
	}
	id := m.saves[m.saveSel].ID
	f, path, err := m.store.Recover(id)
	if err != nil {
		return m, err
	}
	next, err := m.adoptRun(id, f)
	if err != nil {
		return m, err
	}
	next.status = "Carrying on from the backup " + path + ". The damaged save is still on disk; the next write replaces it."
	return next, nil
}

// adoptRun puts a loaded save on screen: the run comes from the save's own trace,
// the claim on the run is taken before it is playable, and the action history is
// restored with it so the next save keeps the run's full history.
func (m Model) adoptRun(id string, f save.File) (Model, error) {
	run, err := f.Run()
	if err != nil {
		return m, err
	}
	lock, err := m.store.Hold(id)
	if err != nil {
		return m, err
	}
	m.run = run
	m.seed = run.Config().Seed
	m.runID = id
	m.lock = lock
	m.actions = append([]game.Action(nil), f.Trace.Actions...)
	m.mode = modeBoard
	m.sel = 0
	m.saves = nil
	m.dayLog = nil
	m.saveErr = ""
	m.savedTo = saveFileName(m.store, id)
	m.restart = false
	m.status = "Carrying on: seed " + itoa(int(m.seed)) + ", day " + itoa(run.State.Day) + "."
	m.statusBad = false
	return m, nil
}
func (m *Model) releaseSave() {
	if m.lock != nil {
		_ = m.lock.Release()
		m.lock = nil
	}
	m.released = true
}

// QuitNote is what the program printed on the way out, for the CLI to show after
// the alternate screen is restored.
func (m Model) QuitNote() string { return m.quitNote }

// StartPersistence offers existing saves before writing a new initial state.
func (m Model) StartPersistence(t SaveTarget) Model {
	m.store, m.runID = t.Store, t.RunID
	if next, offered := m.OfferContinue(); offered {
		return next
	}
	return m.writeSave()
}

// Close releases the writer claim when the program exits unexpectedly.
func (m Model) Close() { m.releaseSave() }
