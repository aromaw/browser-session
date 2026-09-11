package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aromaw/browser-session/internal/browser"
	"github.com/aromaw/browser-session/internal/platform"
)

type Session struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Browser   browser.Browser `json:"browser"`
	CreatedAt time.Time       `json:"createdAt"`
}
type Config struct {
	Version  int       `json:"version"`
	Sessions []Session `json:"sessions"`
}
type Store struct{ Root string }

var validName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)
var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func NewStore(root string) (*Store, error) {
	var e error
	if root == "" {
		root, e = platform.DataHome()
		if e != nil {
			return nil, e
		}
	}
	root, e = filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	if root == filepath.VolumeName(root)+string(filepath.Separator) {
		return nil, errors.New("data root cannot be a filesystem root")
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return nil, e
	}
	if e = platform.SecureDir(root); e != nil {
		return nil, e
	}
	s := &Store{root}
	for _, d := range []string{"sessions", "locks"} {
		if e = s.mkdir(d); e != nil {
			return nil, e
		}
	}
	return s, nil
}
func (s *Store) safe(parts ...string) (string, error) {
	p := s.Root
	for _, part := range parts {
		if part == "" || filepath.Base(part) != part || part == "." || part == ".." {
			return "", errors.New("unsafe path component")
		}
		p = filepath.Join(p, part)
		info, e := os.Lstat(p)
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlink in managed path")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return p, nil
}
func (s *Store) mkdir(parts ...string) error {
	p, e := s.safe(parts...)
	if e != nil {
		return e
	}
	return os.MkdirAll(p, 0700)
}
func (s *Store) Dir(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errors.New("invalid session ID")
	}
	return s.safe("sessions", id, "browser-data")
}
func (s *Store) sessionPath(id, file string) (string, error) {
	if !validID.MatchString(id) {
		return "", errors.New("invalid session ID")
	}
	return s.safe("sessions", id, file)
}
func (s *Store) lock(name string) (*platform.Lock, error) {
	p, e := s.safe("locks", name+".lock")
	if e != nil {
		return nil, e
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		l, ok, e := platform.TryLock(p)
		if e != nil {
			return nil, e
		}
		if ok {
			return l, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("session manager is busy; retry shortly")
		}
		time.Sleep(25 * time.Millisecond)
	}
}
func (s *Store) runLock(id string) (*platform.Lock, bool, error) {
	if !validID.MatchString(id) {
		return nil, false, errors.New("invalid ID")
	}
	p, e := s.safe("locks", id+".run")
	if e != nil {
		return nil, false, e
	}
	return platform.TryLock(p)
}
func (s *Store) Read() (Config, error) {
	c := Config{Version: 1, Sessions: []Session{}}
	p, e := s.safe("sessions.json")
	if e != nil {
		return c, e
	}
	if e = readJSON(p, &c); os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, fmt.Errorf("sessions.json is invalid; preserved without changes: %w", e)
	}
	if c.Version != 1 {
		return c, errors.New("unsupported sessions.json version")
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, v := range c.Sessions {
		if !validID.MatchString(v.ID) || !validName.MatchString(v.Name) || ids[v.ID] || names[v.Name] || (v.Type != "persistent" && v.Type != "temporary") {
			return c, errors.New("invalid or duplicate session metadata")
		}
		ids[v.ID] = true
		names[v.Name] = true
	}
	return c, nil
}
func (s *Store) save(c Config) error {
	p, e := s.safe("sessions.json")
	if e != nil {
		return e
	}
	return atomicJSON(p, c)
}
func atomicJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	f, e := os.CreateTemp(filepath.Dir(path), ".write-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return platform.Replace(tmp, path)
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func find(c Config, name string) (Session, error) {
	for _, v := range c.Sessions {
		if v.Name == name || v.ID == name {
			return v, nil
		}
	}
	return Session{}, fmt.Errorf("session %q not found", name)
}
func (s *Store) Create(name, kind, choice string) (Session, error) {
	if !validName.MatchString(name) {
		return Session{}, errors.New("name must match [a-z][a-z0-9_-]{0,47}")
	}
	if kind != "persistent" && kind != "temporary" {
		return Session{}, errors.New("invalid session type")
	}
	b, e := (browser.Launcher{}).ResolveExecutable(choice)
	if e != nil {
		return Session{}, e
	}
	l, e := s.lock("store")
	if e != nil {
		return Session{}, e
	}
	defer l.Close()
	c, e := s.Read()
	if e != nil {
		return Session{}, e
	}
	if _, e = find(c, name); e == nil {
		return Session{}, errors.New("session name already exists")
	}
	v := Session{ID: ID(), Name: name, Type: kind, Browser: b, CreatedAt: time.Now().UTC()}
	if e = s.mkdir("sessions", v.ID, "browser-data"); e != nil {
		return Session{}, e
	}
	c.Sessions = append(c.Sessions, v)
	if e = s.save(c); e != nil {
		return Session{}, e
	}
	if kind == "temporary" {
		// Reserve the gap between create and open against concurrent cleanup.
		if e = s.writeRun(v.ID, Run{Phase: "reserved", Started: time.Now().UTC()}); e != nil {
			return Session{}, e
		}
	}
	return v, nil
}
func (s *Store) removeData(v Session) error {
	dir, e := s.Dir(v.ID)
	if e != nil {
		return e
	}
	cache := platform.CacheDir(dir)
	if !platform.SamePath(cache, dir) {
		// Reject aliases at every existing cache ancestor before removing anything.
		for p := cache; p != filepath.Dir(p); p = filepath.Dir(p) {
			info, err := os.Lstat(p)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				return errors.New("refusing symlinked external cache")
			}
		}
		if e = os.RemoveAll(cache); e != nil {
			return e
		}
	}
	parent := filepath.Dir(dir)
	if !strings.HasPrefix(parent, s.Root+string(filepath.Separator)) {
		return errors.New("unsafe removal")
	}
	return os.RemoveAll(parent)
}
