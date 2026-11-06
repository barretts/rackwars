// Package save writes a RACK WARS run to disk and reads it back.
//
// The file is JSON, and it holds two things on purpose: the exact State to
// resume from, and the action Trace that produced it. The state makes resuming
// cheap and reroll-proof - quotes and stock are restored as they were, never
// redrawn - while the trace makes the run reproducible: reloading cannot change
// what happened, and load re-runs the trace and requires it to agree with the
// stored state, so an edited or half-written file is caught instead of played.
//
// Nothing in here renders a screen or advances a day. Errors are typed values
// carrying a full sentence, because the TUI prints them verbatim and a player
// decides what to do next: retry, recover the last good save, or start over.
package save

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

// Prefix and Suffix name save files, and LockSuffix the writer lock beside them.
const (
	Prefix     = "run-"
	Suffix     = ".json"
	BakSuffix  = ".bak"
	TmpSuffix  = ".tmp"
	LockSuffix = ".lock"
)

// StaleLock is how long a writer lock may go without a heartbeat before another
// instance may take it over. Autosaves refresh it on every accepted action, so a
// lock this old means the owning process died, slept, or was left open on a
// paused terminal - and the checksum guard below still stops either instance
// from silently overwriting the other's work.
const StaleLock = 45 * time.Minute

// NotFound is returned when a save simply is not there.
var NotFound = errors.New("no such save")

// Failure is any save-layer problem. Every error this package returns is one of
// these, and Message is written for a player, not a log line.
type Failure struct {
	Kind    string
	Path    string
	Message string
	Err     error
}

func (f *Failure) Error() string { return f.Message }
func (f *Failure) Unwrap() error { return f.Err }

func failure(kind, path, format string, args ...any) *Failure {
	return &Failure{Kind: kind, Path: path, Message: fmt.Sprintf(format, args...), Err: nil}
}

// Dir returns the directory runs are saved in: the override if one was given,
// otherwise the platform's per-user configuration directory under "rackwars".
// os.UserConfigDir defines the roots - %AppData% (Roaming Application Data) on
// Windows, ~/Library/Application Support on macOS, and $XDG_CONFIG_HOME or
// ~/.config on Unix. filepath.Join does the joining, so no separator is guessed
// at.
func Dir(override string) (string, error) {
	if override != "" {
		cleaned := filepath.Clean(override)
		if err := checkPath(cleaned); err != nil {
			return "", fmt.Errorf("the save folder %q is not usable: %w", override, err)
		}
		return cleaned, nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot find this system's configuration directory: %w", err)
	}
	return filepath.Join(root, "rackwars", "saves"), nil
}

// checkPath rejects the characters a folder name may not contain: the NUL byte
// everywhere, and < > | ? " * which Windows reserves. Both target platforms get
// the same rule, so a --save-dir that works on one is never a mystery on the
// other: the bad folder is refused here, in words the player can act on, rather
// than arriving later as an unexplained write failure in the middle of a run.
func checkPath(path string) error {
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("it contains a character that cannot appear in a path")
	}
	if strings.ContainsAny(path, `<>|?"*`) {
		return fmt.Errorf("a folder name cannot contain < > | ? \" *")
	}
	return nil
}

// File is a versioned snapshot and the action trace that produced it.
const SchemaVersion = 1
const EngineVersion = "prototype-1"
const ContentVersion = "campaign-1"

type File struct {
	Schema       int    `json:"schemaVersion"`
	RulesVersion string `json:"rulesVersion"`
	Content      string `json:"contentVersion"`
	// Engine records which build wrote the file. It is metadata for a bug
	// report, never a compatibility test.
	Engine  string `json:"engineVersion,omitempty"`
	Written string `json:"written"`

	// Trace is the config the run was dealt from plus every accepted action, in
	// order: the whole explanation of how this state came about, and the thing a
	// bug report attaches.
	Trace game.Trace `json:"trace"`
	// State is the resume point: day, town, cash, debt, inventory, stock, the
	// last quotes seen in every town, run statistics, and any ending.
	State game.State `json:"state"`

	// Pending names something the run is waiting on. The trading prototype has
	// nothing of the kind; the field exists so a later ruleset that stops the
	// morning on an encounter cannot resume past it, and so load can refuse a
	// save whose pending choice this build cannot present.
	Pending string `json:"pending,omitempty"`
}

// Of captures a run plus the actions that got it there.
func Of(run *game.Run, actions []game.Action, engine string) File {
	return File{
		Schema: SchemaVersion, RulesVersion: EngineVersion, Content: ContentVersion,
		Engine:  engine,
		Written: time.Now().Format(time.RFC3339),
		Trace:   game.Trace{Config: run.Config(), Actions: append([]game.Action(nil), actions...)},
		State:   run.State,
	}
}

// Run rebuilds the playable run from the file. No prices are redrawn.
func (f File) Run() (*game.Run, error) {
	return game.Resume(f.Trace.Config, f.State)
}

// Store reads and writes save files in one directory.
type Store struct {
	Dir    string
	Engine string

	// written maps a save path to the checksum of the bytes this instance last
	// wrote there, which is how an outside change gets noticed instead of
	// quietly replaced on the next autosave.
	written map[string]string
}

// NewStore prepares dir for saving. The check is a real write, so a directory
// that cannot be used fails here, in front of the player, rather than on the
// twentieth autosave.
func NewStore(dir, engine string) (*Store, error) {
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, failure("unusable-dir", dir,
			"cannot use %s as the save folder: %v", dir, err)
	}
	probe := filepath.Join(dir, "rackwars-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		return nil, failure("unusable-dir", dir,
			"cannot write in %s: %v", dir, err)
	}
	_ = os.Remove(probe)
	return &Store{Dir: dir, Engine: engine, written: map[string]string{}}, nil
}

// SafeID reports whether id is safe to use as the middle of a save file name.
// Run ids come from the seed or a player's label, so the character set is
// deliberately narrow: only [A-Za-z0-9._-], with the first and last characters
// alphanumeric, which rules out the traversal and dot-only names that would
// otherwise escape or hide a save file. Anything else is refused with a reason,
// and the caller never builds a path from unvalidated text.
func SafeID(id string) error {
	if id == "" || len(id) > 64 {
		return fmt.Errorf("run id %q must be between 1 and 64 characters", id)
	}
	if !alphanumeric(id[0]) || !alphanumeric(id[len(id)-1]) {
		return fmt.Errorf("run id %q must start and end with a letter or a digit", id)
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
		default:
			return fmt.Errorf("run id %q may only contain letters, digits, dot, dash and underscore", id)
		}
	}
	return nil
}

func alphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Path is the active save file for a run id.
func (s *Store) Path(id string) (string, error) {
	if err := SafeID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, Prefix+id+Suffix), nil
}

// TracePaths are the two files an exported trace is written to.
func (s *Store) TracePaths(id string) (string, string, error) {
	if err := SafeID(id); err != nil {
		return "", "", err
	}
	base := filepath.Join(s.Dir, "trace-"+id)
	return base + Suffix, base + ".txt", nil
}

// Lock keeps one instance's claim on a run. The holder writes its pid and host
// so a second instance can say who else has it open.
type Lock struct {
	Path   string
	Holder string
	// Note explains an oddity worth telling the player about, such as taking a
	// lock over a process that clearly died. It is empty in the normal case.
	Note string

	released bool
}

// Hold claims a run. A fresh lock held by somebody else is refused; a lock whose
// heartbeat has gone stale is taken over, with the fact recorded in Note.
func (s *Store) Hold(id string) (*Lock, error) {
	path, err := s.Path(id)
	if err != nil {
		return nil, err
	}
	lockPath := path + LockSuffix
	holder := fmt.Sprintf("pid %d on %s", os.Getpid(), hostname())
	take := func() (*Lock, error) {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		_, _ = f.WriteString(holder + "\n" + time.Now().Format(time.RFC3339) + "\n")
		_ = f.Close()
		return &Lock{Path: lockPath, Holder: holder}, nil
	}
	l, err := take()
	if err == nil {
		s.recordExisting(path)
		return l, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		// The folder cannot hold the marker at all, which is the same problem as a
		// failed write: the run is playable but not on disk, and the player needs
		// the same two ways out.
		return nil, &Failure{Kind: "unusable-dir", Path: lockPath, Err: err,
			Message: fmt.Sprintf("cannot write the save marker %s: %v. The run is still in memory; press S to try again "+
				"or restart with --save-dir pointing somewhere writable.", lockPath, err)}
	}
	who, when := readLock(lockPath)
	if !when.IsZero() && time.Since(when) < StaleLock && !who.empty {
		return nil, &Failure{
			Kind: "busy", Path: path, Err: err,
			Message: fmt.Sprintf("%s is already open by %s; close that RACK WARS window before opening this run again.",
				path, who.text),
		}
	}
	// Stale or unreadable. A marker can also look empty in the moment another
	// window is creating or refreshing it, so before deciding the owner is gone,
	// look once more after a beat: stealing the marker of a live window is the
	// worse mistake, and a marker that only looked dead because somebody was
	// mid-rewrite looks complete again by now.
	time.Sleep(100 * time.Millisecond)
	if who2, when2 := readLock(lockPath); !when2.IsZero() && time.Since(when2) < StaleLock && !who2.empty {
		return nil, &Failure{
			Kind: "busy", Path: path, Err: err,
			Message: fmt.Sprintf("%s is already open by %s; close that RACK WARS window before opening this run again.",
				path, who2.text),
		}
	} else if !who2.empty || !when2.IsZero() {
		who = who2
	}
	// Stale or unreadable after the second look: remove and claim it once more.
	// If the claim still fails, another instance won the race and refusing is
	// correct.
	_ = os.Remove(lockPath)
	l, err = take()
	if err != nil {
		return nil, &Failure{
			Kind: "busy", Path: path, Err: err,
			Message: fmt.Sprintf("%s is already open by another RACK WARS instance; only one window may save a run at a time.", path),
		}
	}
	s.recordExisting(path)
	return &Lock{Path: lockPath, Holder: holder,
		Note: fmt.Sprintf("took over the save marker left by %s", who.text)}, nil
}

// recordExisting baselines a save file that is already on disk the moment a
// window claims the run. The clobber guard can only compare against a hash this
// store has recorded, so without a claim-time baseline a window that claimed a
// run it never loaded would sit outside the guard entirely: the first write the
// other window lands after the takeover would pass the blind window's guard,
// not trip it. With the baseline, any file movement the holder makes after the
// claim is exactly what the blind window's next save refuses to overwrite, and
// the holder keeps sole write authority. A takeover writer can still replace a
// file that has not moved since the claim (its position is one the player could
// have continued into, which is what a takeover is for), and that replacement
// keeps the previous file as the .bak copy the continue list recovers from.
func (s *Store) recordExisting(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	if h, err := hashOf(path); err == nil {
		s.written[path] = h
	}
}

// Touch refreshes the heartbeat; Save calls it. The rewrite goes through a
// sibling temp file and a rename, never a truncate of the marker in place: a
// reader that caught the marker mid-truncate would see an empty file, decide a
// live window had died, and steal its run. A rename shows readers either the
// old marker whole or the new one whole.
func (l *Lock) Touch() error {
	if l == nil || l.released {
		return nil
	}
	buf := []byte(l.Holder + "\n" + time.Now().Format(time.RFC3339) + "\n")
	tmp := l.Path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := writeSynced(tmp, buf); err != nil {
		_ = os.Remove(tmp)
		return &Failure{Kind: "lock", Path: l.Path, Err: err,
			Message: "the marker saying this run is open could not be refreshed; another window may be able to open it too."}
	}
	if err := replaceWithRetry(tmp, l.Path); err != nil {
		_ = os.Remove(tmp)
		return &Failure{Kind: "lock", Path: l.Path, Err: err,
			Message: "the marker saying this run is open could not be refreshed; another window may be able to open it too."}
	}
	return nil
}

// Release gives up the claim.
func (l *Lock) Release() error {
	if l == nil || l.released {
		return nil
	}
	l.released = true
	if err := os.Remove(l.Path); err != nil && !os.IsNotExist(err) {
		return &Failure{Kind: "lock", Path: l.Path, Err: err,
			Message: "the marker saying this run is open could not be removed; a later window may think the run is still open."}
	}
	return nil
}

type lockOwner struct {
	text  string
	empty bool
}

func readLock(path string) (owner lockOwner, when time.Time) {
	b, err := os.ReadFile(path)
	if err != nil {
		return lockOwner{text: "another instance", empty: true}, time.Time{}
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	who := lockOwner{text: "another instance", empty: true}
	if len(lines) > 0 && lines[0] != "" {
		who = lockOwner{text: lines[0], empty: false}
	}
	if len(lines) > 1 {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[1])); err == nil {
			when = t
		}
	}
	return who, when
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "this machine"
	}
	return h
}

// Save writes the run to its active file, keeping the previous version as a
// .bak, and only then confirms the write by reading it back. A failure leaves
// the run running in memory; the caller shows Message and may retry.
//
// The order is deliberate, because Windows is not Unix about replacing a file
// somebody might still have open:
//
//  1. refuse to overwrite a file another program changed since our last write;
//  2. write a sibling temp file, flush it to disk, close it;
//  3. copy the current file aside to .bak;
//  4. rename the temp file over the active one, retrying a sharing violation a
//     few times, since an antivirus or indexer can hold the target open for a
//     moment;
//  5. read the result back and verify it is the run we meant to store.
func (s *Store) Save(id string, f File, lock *Lock) error { return s.save(id, f, lock, false) }

// Restart is Save without the refusal to move a run backwards in time, for the
// one case that is deliberate: the player chose to start this seed over. The
// previous run is still kept in the .bak copy, and the guard against another
// window's write still applies.
func (s *Store) Restart(id string, f File, lock *Lock) error { return s.save(id, f, lock, true) }

func (s *Store) save(id string, f File, lock *Lock, restart bool) error {
	path, err := s.Path(id)
	if err != nil {
		return err
	}
	f.Engine = s.Engine
	if f.Written == "" {
		f.Written = time.Now().Format(time.RFC3339)
	}
	if err := s.writable(f, path); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return failure("encode", path, "could not write the run out: %v", err)
	}
	sum := sha256.Sum256(buf)

	if _, err := os.Stat(path); err == nil {
		if prev, ok := s.written[path]; ok {
			cur, err := hashOf(path)
			if err != nil {
				return &Failure{Kind: "write", Path: path, Err: err,
					Message: fmt.Sprintf("cannot read the existing save %s: %v. Nothing was overwritten.", path, err)}
			}
			if cur != prev {
				return &Failure{Kind: "clobbered", Path: path, Err: errors.New("save changed on disk"),
					Message: fmt.Sprintf("%s changed since this window last saved it, so nothing was overwritten. "+
						"Another program or window wrote it; continue from that file, or start a new run.", path)}
			}
		}
		// A save that moves a run backwards in time is refused unless the caller
		// asked for it by name (Restart, below). Autosave never deliberately rewinds
		// a run, so a backwards write means a bug or a collision with another run in
		// the same slot, and losing progress is the worst thing a save layer can do.
		if !restart {
			if back, err := s.load(path); err == nil && back.State.Day > f.State.Day {
				return &Failure{Kind: "backwards", Path: path,
					Err: fmt.Errorf("saved run is on day %d, the incoming run is on day %d", back.State.Day, f.State.Day),
					Message: fmt.Sprintf("%s already holds that run on day %d, further than day %d, so nothing was "+
						"overwritten. Continue from the saved run, or choose to start the seed over.", path, back.State.Day, f.State.Day)}
			}
		}
		if _, valid := s.load(path); valid == nil {
			if err := copyAside(path, path+BakSuffix); err != nil {
				return &Failure{Kind: "backup", Path: path, Err: err,
					Message: fmt.Sprintf("could not keep the previous copy of %s: %v. Nothing was overwritten.", path, err)}
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return &Failure{Kind: "write", Path: path, Err: err,
			Message: fmt.Sprintf("cannot read the existing save %s: %v. Nothing was overwritten.", path, err)}
	}

	tmp := filepath.Join(s.Dir, Prefix+id+TmpSuffix+"-"+strconv.Itoa(os.Getpid()))
	if err := writeSynced(tmp, buf); err != nil {
		_ = os.Remove(tmp)
		return &Failure{Kind: "write", Path: path, Err: err,
			Message: fmt.Sprintf("cannot write the save into %s: %v. The run is still in memory; press S to try again "+
				"or restart with --save-dir pointing somewhere writable.", s.Dir, err)}
	}
	if err := replaceWithRetry(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return &Failure{Kind: "write", Path: path, Err: err,
			Message: fmt.Sprintf("cannot move the new save into place over %s: %v. The run is still in memory; "+
				"press S to try again or restart with --save-dir pointing somewhere writable.", path, err)}
	}
	s.written[path] = hex.EncodeToString(sum[:])
	if err := lock.Touch(); err != nil {
		return err
	}

	back, err := s.load(path)
	if err != nil {
		return &Failure{Kind: "verify", Path: path, Err: err,
			Message: fmt.Sprintf("the save written to %s did not read back correctly: %v. The previous copy is at %s.",
				path, err, path+BakSuffix)}
	}
	if back.State.Day != f.State.Day || back.State.Cash != f.State.Cash || back.State.Debt != f.State.Debt {
		return &Failure{Kind: "verify", Path: path, Err: errors.New("saved run does not match memory"),
			Message: fmt.Sprintf("the save in %s does not match the run in memory; nothing was accepted. "+
				"The previous copy is at %s.", path, path+BakSuffix)}
	}
	return nil
}

// Export writes a trace out as both JSON and the compact text form, so a bug
// report can attach the one that reads well and keep the one that replays.
func (s *Store) Export(id string, tr game.Trace) (string, string, error) {
	jsonPath, txtPath, err := s.TracePaths(id)
	if err != nil {
		return "", "", err
	}
	run, steps, err := tr.Replay()
	if err != nil {
		return "", "", failure("trace", jsonPath, "this run's actions do not replay: %v", err)
	}
	buf, err := game.EncodeTrace(tr)
	if err != nil {
		return "", "", failure("encode", jsonPath, "could not write the trace: %v", err)
	}
	if err := writeSynced(jsonPath, buf); err != nil {
		return "", "", failure("write", jsonPath, "cannot write %s: %v", jsonPath, err)
	}
	if err := writeSynced(txtPath, []byte(game.TextTrace(run, steps))); err != nil {
		return "", "", failure("write", txtPath, "cannot write %s: %v", txtPath, err)
	}
	return jsonPath, txtPath, nil
}

// LoadTrace reads a trace file back for replay.
func (s *Store) LoadTrace(path string) (game.Trace, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return game.Trace{}, &Failure{Kind: "not-found", Path: path, Err: err,
			Message: fmt.Sprintf("cannot read the trace %s: %v", path, err)}
	}
	tr, err := game.DecodeTrace(b)
	if err != nil {
		return game.Trace{}, &Failure{Kind: "corrupt", Path: path, Err: err,
			Message: fmt.Sprintf("%s is not a readable trace: %v", path, err)}
	}
	return tr, nil
}

// Load reads and validates a run.
func (s *Store) Load(id string) (File, error) {
	path, err := s.Path(id)
	if err != nil {
		return File{}, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return File{}, &Failure{Kind: "not-found", Path: path, Err: err,
				Message: fmt.Sprintf("there is no saved run in %s yet.", s.Dir)}
		}
		return File{}, failure("read", path, "cannot open %s: %v", path, err)
	}
	return s.load(path)
}

func (s *Store) load(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, failure("read", path, "cannot open %s: %v", path, err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, &Failure{Kind: "corrupt", Path: path, Err: err,
			Message: fmt.Sprintf("%s is damaged: the file is not readable as a saved run (%v). "+
				"Recover the last good copy, or start a new run.", path, err)}
	}
	if err := s.check(f, path); err != nil {
		return File{}, err
	}
	s.written[path], _ = hashOf(path)
	return f, nil
}

// Recover returns the last good copy of a run. The active file is tried first;
// if it is damaged, the .bak is read instead, and a run whose backup also fails
// reports both paths rather than pretending nothing happened.
func (s *Store) Recover(id string) (File, string, error) {
	path, err := s.Path(id)
	if err != nil {
		return File{}, "", err
	}
	if f, err := s.load(path); err == nil {
		return f, path, nil
	} else if _, statErr := os.Stat(path); statErr != nil {
		return File{}, "", &Failure{Kind: "not-found", Path: path, Err: err,
			Message: fmt.Sprintf("there is no saved run at %s to recover.", path)}
	}
	bak := path + BakSuffix
	if _, err := os.Stat(bak); err != nil {
		return File{}, "", &Failure{Kind: "no-backup", Path: bak, Err: err,
			Message: fmt.Sprintf("%s is damaged and there is no backup copy next to it. Start a new run, "+
				"or point --save-dir at the folder holding an older save.", path)}
	}
	f, err := s.load(bak)
	if err != nil {
		return File{}, "", &Failure{Kind: "no-backup", Path: bak, Err: err,
			Message: fmt.Sprintf("neither %s nor its backup %s can be read. Start a new run, or use --save-dir "+
				"to point at a folder with an older save.", path, bak)}
	}
	s.written[path], _ = hashOf(path)
	return f, bak, nil
}

// Entry is one saved run as the Continue prompt shows it.
type Entry struct {
	ID      string
	Path    string
	Seed    uint64
	Day     int
	Market  content.MarketID
	Cash    game.Money
	Debt    game.Money
	Over    game.EndKind
	Written string
	// Err is set when the file exists but cannot be used; the prompt says so
	// instead of dropping the row silently.
	Err error
}

// List returns saved runs, newest file name first. Damaged files are listed with
// their problem so the player can see why Continue will not offer them.
func (s *Store) List() ([]Entry, error) {
	matches, err := filepath.Glob(filepath.Join(s.Dir, Prefix+"*"+Suffix))
	if err != nil {
		return nil, failure("list", s.Dir, "cannot list the save folder %s: %v", s.Dir, err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	out := make([]Entry, 0, len(matches))
	for _, m := range matches {
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(m), Prefix), Suffix)
		e := Entry{ID: id, Path: m}
		f, err := s.load(m)
		if err != nil {
			e.Err = err
			out = append(out, e)
			continue
		}
		e.Day, e.Market, e.Cash, e.Debt, e.Over, e.Written =
			f.State.Day, f.State.Market, f.State.Cash, f.State.Debt, f.State.Over, f.Written
		e.Seed = f.Trace.Config.Seed
		out = append(out, e)
	}
	return out, nil
}

// writable is the check applied before a file is written: the state the caller
// hands over must be a run this engine could actually be standing in. That stops
// a programming error from reaching the disk as a save file. The recorded history
// is deliberately not replayed here - the caller is the one that just performed
// those actions, and replaying on every autosave would burn CPU on the trusted
// path. Untrusted input is checked on the way in, by check.
func (s *Store) writable(f File, path string) error {
	if _, err := game.Resume(f.Trace.Config, f.State); err != nil {
		return &Failure{Kind: "internal", Path: path, Err: err,
			Message: fmt.Sprintf("refused to write %s: the run in memory is not a legal position (%v). "+
				"Nothing on disk was changed.", path, err)}
	}
	return nil
}

// check is the read path: everything writable does, plus the two rules that only
// apply to a file somebody else produced. The recorded actions must replay to the
// stored state, which is what makes a reload deterministic instead of a reroll,
// and a save waiting on a choice this build cannot show must not be resumed past
// it. Both failures come with the recovery sentence the player needs, not a
// stack trace.
func (s *Store) check(f File, path string) error {
	if f.Schema != SchemaVersion || f.RulesVersion != EngineVersion || f.Content != ContentVersion {
		return failure("incompatible", path, "%s uses an unsupported save, engine, or content version. Keep the file and use the matching build; no automatic migration is available.", path)
	}
	if _, err := game.Resume(f.Trace.Config, f.State); err != nil {
		return &Failure{Kind: "corrupt", Path: path, Err: err,
			Message: fmt.Sprintf("%s does not hold a legal run: %v. Recover the last good copy, or start a new run.",
				path, err)}
	}
	if f.Pending != "" {
		return &Failure{Kind: "pending", Path: path,
			Message: fmt.Sprintf("%s stops on a choice this build cannot show (%s); the run cannot be resumed safely. "+
				"Recover the last good copy, or start a new run.", path, f.Pending)}
	}
	if why, err := traceAgrees(f); err != nil {
		return &Failure{Kind: "drift", Path: path, Err: err,
			Message: fmt.Sprintf("%s does not add up: %s. Recover the last good copy, or start a new run.", path, why)}
	}
	return nil
}

// traceAgrees replays the recorded actions and compares the result to the state
// in the file. Silence means the save is exactly what its own history produced;
// a mismatch means the file was edited, truncated, or written by a build whose
// rules differ, and replaying it would change the player's situation.
func traceAgrees(f File) (string, error) {
	run, steps, err := f.Trace.Replay()
	if err != nil {
		return fmt.Sprintf("the recorded actions do not replay (%v)", err), err
	}
	if len(steps) != len(f.Trace.Actions) {
		return fmt.Sprintf("the engine refused action %d of %d that the save recorded as accepted",
			len(steps), len(f.Trace.Actions)), errors.New("refused action in trace")
	}
	got := run.State
	want := f.State
	if d := firstStateDifference(got, want); d.label != "" {
		return fmt.Sprintf("playing back the %d recorded actions leaves %s of %s, but the file stores %s",
			len(steps), d.label, d.got, d.want), errors.New(d.label)
	}
	return "", nil
}

type diff struct{ label, got, want string }

func firstStateDifference(got, want game.State) diff {
	switch {
	case got.Day != want.Day:
		return diff{"day", strconv.Itoa(got.Day), strconv.Itoa(want.Day)}
	case got.Market != want.Market:
		return diff{"market", string(got.Market), string(want.Market)}
	case got.Cash != want.Cash:
		return diff{"cash", game.FormatMoney(got.Cash), game.FormatMoney(want.Cash)}
	case got.Debt != want.Debt:
		return diff{"debt", game.FormatMoney(got.Debt), game.FormatMoney(want.Debt)}
	case got.Over != want.Over, got.End != want.End:
		return diff{"ending", got.End.Kind.String(), want.End.Kind.String()}
	case !equalInts(got.Owned, want.Owned):
		return diff{"inventory", intsLabel(got.Owned), intsLabel(want.Owned)}
	case !equalInts(got.Stock, want.Stock):
		return diff{"stock", intsLabel(got.Stock), intsLabel(want.Stock)}
	case !equalQuotes(got.Quotes, want.Quotes):
		return diff{"prices", quotesLabel(got.Quotes), quotesLabel(want.Quotes)}
	case got.Stats != want.Stats:
		return diff{"run statistics", statsLabel(got.Stats), statsLabel(want.Stats)}
	}
	return diff{}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalQuotes(a, b []game.Quote) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func intsLabel(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return "[" + strings.Join(s, " ") + "]"
}

func quotesLabel(v []game.Quote) string {
	if len(v) == 0 {
		return "[]"
	}
	q := v[0]
	return fmt.Sprintf("first cell %s ask / %s bid", game.FormatMoney(q.Ask), game.FormatMoney(q.Bid))
}

func statsLabel(s game.Stats) string {
	return fmt.Sprintf("interest %s, commitments %d", game.FormatMoney(s.InterestPaid), s.Commitments)
}

// writeSynced writes a file and asks the OS to get it onto the platter before
// returning, so a rename can not name a file whose contents are still in a
// buffer.
func writeSynced(path string, buf []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// copyAside preserves the active file until its replacement succeeds.
func copyAside(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return writeSynced(to, b)
}

// replaceWithRetry renames tmp over target, retrying the transient sharing
// violation Windows raises when another process has the target open. A failure
// after the retries is reported rather than worked around by deleting the
// active save first, which would trade a failure for a hole.
func replaceWithRetry(tmp, target string) error {
	var last error
	for attempt := 0; attempt < 6; attempt++ {
		if err := os.Rename(tmp, target); err == nil {
			return nil
		} else {
			last = err
		}
		time.Sleep(time.Duration(10*(1<<attempt)) * time.Millisecond)
	}
	return last
}

func hashOf(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
