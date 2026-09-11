package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/aromaw/browser-session/internal/browser"
	"github.com/aromaw/browser-session/internal/platform"
)

type Run struct {
	ID        string             `json:"id"`
	Phase     string             `json:"phase"`
	Boot      string             `json:"boot"`
	Started   time.Time          `json:"started"`
	Group     int                `json:"group,omitempty"`
	Processes []platform.Process `json:"processes,omitempty"`
	Error     string             `json:"error,omitempty"`
}

func (s *Store) readRun(id string) (Run, error) {
	var r Run
	p, e := s.sessionPath(id, "run.json")
	if e != nil {
		return r, e
	}
	e = readJSON(p, &r)
	return r, e
}
func (s *Store) writeRun(id string, r Run) error {
	p, e := s.sessionPath(id, "run.json")
	if e != nil {
		return e
	}
	return atomicJSON(p, r)
}
func (s *Store) Lookup(name string) (Session, error) {
	c, e := s.Read()
	if e != nil {
		return Session{}, e
	}
	return find(c, name)
}
func (s *Store) active(v Session, r Run) ([]platform.Process, error) {
	dir, e := s.Dir(v.ID)
	if e != nil {
		return nil, e
	}
	boot, e := platform.BootID()
	if e != nil {
		return nil, e
	}
	if r.Boot != boot {
		r.Group = 0
		r.Processes = nil
	}
	return (browser.Launcher{}).IsRunning(v.Browser.Executable, dir, r.Processes, r.Group)
}
func (s *Store) Status(v Session) string {
	l, ok, e := s.runLock(v.ID)
	if e != nil {
		return "unknown"
	}
	if !ok {
		return "running"
	}
	l.Close()
	r, e := s.readRun(v.ID)
	if e != nil && !os.IsNotExist(e) {
		return "unknown"
	}
	if (r.Phase == "starting" || r.Phase == "reserved") && time.Since(r.Started) < 30*time.Second {
		return "starting"
	}
	p, e := s.active(v, r)
	if e != nil {
		return "unknown"
	}
	if len(p) > 0 {
		return "unmanaged"
	}
	if r.Error != "" {
		return "stopped (see status)"
	}
	return "stopped"
}
func (s *Store) Open(name string, urls []string) (Session, error) {
	if e := browser.ValidateURLs(urls); e != nil {
		return Session{}, e
	}
	l, e := s.lock("store")
	if e != nil {
		return Session{}, e
	}
	v, e := s.Lookup(name)
	if e != nil {
		l.Close()
		return v, e
	}
	runLock, ok, e := s.runLock(v.ID)
	if e != nil {
		l.Close()
		return v, e
	}
	if !ok {
		l.Close()
		return v, errors.New("session is already running")
	}
	r, err := s.readRun(v.ID)
	if err != nil && !os.IsNotExist(err) {
		runLock.Close()
		l.Close()
		return v, err
	}
	if r.Phase == "starting" && time.Since(r.Started) < 30*time.Second {
		runLock.Close()
		l.Close()
		return v, errors.New("session is already starting")
	}
	p, e := s.active(v, r)
	if e != nil || len(p) > 0 {
		runLock.Close()
		l.Close()
		if e != nil {
			return v, e
		}
		return v, errors.New("session has browser processes; close them first")
	}
	boot, e := platform.BootID()
	if e != nil {
		runLock.Close()
		l.Close()
		return v, e
	}
	r = Run{ID: ID(), Phase: "starting", Started: time.Now().UTC(), Boot: boot}
	e = s.writeRun(v.ID, r)
	runLock.Close()
	l.Close()
	if e != nil {
		return v, e
	}
	exe, e := os.Executable()
	if e != nil {
		return v, e
	}
	args := append([]string{"__supervise", s.Root, v.ID, r.ID}, urls...)
	c := exec.Command(exe, args...)
	platform.Detach(c)
	if e = c.Start(); e != nil {
		r.Phase = "failed"
		r.Error = "could not start supervisor"
		_ = s.writeRun(v.ID, r)
		return v, e
	}
	_ = c.Process.Release()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		current, e := s.readRun(v.ID)
		if os.IsNotExist(e) {
			if _, lookupErr := s.Lookup(v.ID); lookupErr != nil {
				return v, errors.New("browser exited before startup was confirmed; temporary data cleaned")
			}
		}
		if e == nil && current.ID == r.ID {
			if current.Phase == "running" {
				return v, nil
			}
			if current.Phase == "failed" || current.Phase == "stopped" {
				return v, fmt.Errorf("browser did not stay running: %s", current.Error)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return v, errors.New("startup could not be confirmed; use status before retrying (data retained)")
}

// Supervise is an internal entry point of the same binary. The lifetime lock
// prevents cleanup even if a browser wrapper exits while children remain alive.
func (s *Store) Supervise(id, runID string, urls []string) error {
	life, ok, e := s.runLock(id)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("already supervised")
	}
	defer life.Close()
	l, e := s.lock("store")
	if e != nil {
		return e
	}
	v, e := s.Lookup(id)
	if e != nil {
		l.Close()
		return e
	}
	r, e := s.readRun(id)
	if e != nil || r.ID != runID || r.Phase != "starting" {
		l.Close()
		return errors.New("stale launch request")
	}
	dir, e := s.Dir(id)
	if e != nil {
		l.Close()
		return e
	}
	// Detect external launches again while holding both locks.
	live, e := s.active(v, r)
	if e != nil || len(live) > 0 {
		r.Phase = "failed"
		r.Error = "existing processes or inspection failure"
		_ = s.writeRun(id, r)
		l.Close()
		return errors.New(r.Error)
	}
	c, e := (browser.Launcher{}).Launch(v.Browser.Executable, dir, urls)
	if e != nil {
		r.Phase = "failed"
		r.Error = "browser executable could not be started"
		_ = s.writeRun(id, r)
		l.Close()
		return e
	}
	if runtime.GOOS != "windows" {
		r.Group = c.Process.Pid
	}
	all, snapErr := platform.Snapshot()
	if snapErr == nil {
		for _, p := range all {
			if p.PID == c.Process.Pid {
				r.Processes = append(r.Processes, p)
			}
		}
	}
	e = s.writeRun(id, r)
	l.Close()
	// Always reap. Wait returning only means the original child exited.
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	if e != nil {
		return e
	} // browser remains recoverable via directory/process inspection
	exited := false
	var exitErr error
	empty := 0
	closeSent := false
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		select {
		case exitErr = <-done:
			exited = true
			done = nil
		default:
		}
		owned, inspectErr := s.active(v, r)
		if inspectErr != nil {
			r.Error = "process inspection failed; cleanup deferred"
			_ = s.writeRun(id, r)
			continue
		}
		r.Processes = owned
		closePath, e := s.sessionPath(id, "close.json")
		if e != nil {
			return e
		}
		var closeID string
		if !closeSent && readJSON(closePath, &closeID) == nil && closeID == runID {
			if !exited {
				if err := platform.CloseBrowser(c.Process); err != nil {
					r.Error = err.Error()
				}
			}
			closeSent = true
		}
		if exited && len(owned) == 0 {
			empty++
		} else {
			empty = 0
		}
		if empty >= 4 {
			break
		} // one full second with no live processes
		if time.Since(r.Started) >= 750*time.Millisecond && len(owned) > 0 {
			r.Phase = "running"
		}
		if e = s.writeRun(id, r); e != nil {
			return e
		}
	}
	r.Phase = "stopped"
	r.Group = 0
	r.Processes = nil
	if exitErr != nil {
		r.Error = "browser exited unsuccessfully; browser output was not recorded"
	}
	if e = s.writeRun(id, r); e != nil {
		return e
	}
	if v.Type == "temporary" {
		return s.removeStopped(v)
	}
	return nil
}
func (s *Store) removeStopped(v Session) error {
	l, e := s.lock("store")
	if e != nil {
		return e
	}
	defer l.Close()
	return s.removeStoppedLocked(v)
}

func (s *Store) removeStoppedLocked(v Session) error {
	c, e := s.Read()
	if e != nil {
		return e
	}
	if _, e = find(c, v.ID); e != nil {
		return nil
	}
	if e = s.removeData(v); e != nil {
		return e
	}
	remaining := make([]Session, 0, len(c.Sessions))
	for _, x := range c.Sessions {
		if x.ID != v.ID {
			remaining = append(remaining, x)
		}
	}
	c.Sessions = remaining
	return s.save(c)
}
func (s *Store) Close(name string) error {
	v, e := s.Lookup(name)
	if e != nil {
		return e
	}
	life, ok, e := s.runLock(v.ID)
	if e != nil {
		return e
	}
	if ok {
		life.Close()
		return errors.New("no active supervisor; close the session's browser manually, then run cleanup")
	}
	r, e := s.readRun(v.ID)
	if e != nil {
		return e
	}
	p, e := s.sessionPath(v.ID, "close.json")
	if e != nil {
		return e
	}
	if e = atomicJSON(p, r.ID); e != nil {
		return e
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		lock, ok, e := s.runLock(v.ID)
		if e != nil {
			return e
		}
		if ok {
			lock.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("browser is still running; confirm any browser dialog or quit this session manually (data retained)")
}
func (s *Store) Delete(name string) error {
	v, e := s.Lookup(name)
	if e != nil {
		return e
	}
	return s.deleteIfIdle(v, false)
}
func (s *Store) deleteIfIdle(v Session, cleanup bool) error {
	// Lock order is lifetime, then store, matching the supervisor.
	life, ok, e := s.runLock(v.ID)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("session is running")
	}
	defer life.Close()
	// Re-read state under the store lock: create may have been between its
	// metadata commit and temporary reservation when cleanup first saw it.
	l, e := s.lock("store")
	if e != nil {
		return e
	}
	defer l.Close()
	r, e := s.readRun(v.ID)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if (r.Phase == "starting" || cleanup && r.Phase == "reserved") && time.Since(r.Started) < 30*time.Second {
		return errors.New("session is starting")
	}
	for i := 0; i < 2; i++ {
		p, e := s.active(v, r)
		if e != nil {
			return e
		}
		if len(p) > 0 {
			return errors.New("browser processes still use this session")
		}
		if i == 0 {
			time.Sleep(250 * time.Millisecond)
		}
	}
	return s.removeStoppedLocked(v)
}
func (s *Store) Cleanup() []string {
	c, e := s.Read()
	if e != nil {
		return []string{e.Error()}
	}
	var notes []string
	for _, v := range c.Sessions {
		if v.Type == "temporary" {
			if e = s.deleteIfIdle(v, true); e != nil {
				if s.Status(v) != "running" && s.Status(v) != "starting" {
					notes = append(notes, v.Name+": "+e.Error())
				}
			}
		}
	}
	return notes
}
func (s *Store) Detail(name string) (Session, Run, error) {
	v, e := s.Lookup(name)
	if e != nil {
		return v, Run{}, e
	}
	r, e := s.readRun(v.ID)
	if os.IsNotExist(e) {
		e = nil
	}
	return v, r, e
}
func (s *Store) DataPath(v Session) string { p, _ := s.Dir(v.ID); return filepath.Clean(p) }
