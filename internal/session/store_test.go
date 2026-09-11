package session

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := NewStore(filepath.Join(t.TempDir(), "sessions root 中文"))
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func testCreate(t *testing.T, s *Store, name, kind string) Session {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Create(name, kind, exe)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestConcurrentCreate(t *testing.T) {
	s := testStore(t)
	exe, _ := os.Executable()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		wg.Add(1)
		go func(name string) { defer wg.Done(); _, e := s.Create(name, "persistent", exe); errs <- e }(n)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	c, e := s.Read()
	if e != nil || len(c.Sessions) != 8 {
		t.Fatal(c, e)
	}
}
func TestInvalidNamesAndCorruptConfig(t *testing.T) {
	s := testStore(t)
	exe, _ := os.Executable()
	for _, n := range []string{"../oops", "../", "CON", "a/b", "a\\b", "a:b", "", "--flag"} {
		if _, e := s.Create(n, "persistent", exe); e == nil {
			t.Fatal(n)
		}
	}
	p := filepath.Join(s.Root, "sessions.json")
	bad := []byte("{broken")
	if e := os.WriteFile(p, bad, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Create("a", "persistent", exe); e == nil {
		t.Fatal("overwrote broken config")
	}
	got, _ := os.ReadFile(p)
	if string(got) != string(bad) {
		t.Fatal(string(got))
	}
}
func TestDeleteGuardsAndTemporaryRecovery(t *testing.T) {
	if os.Getenv("BROWSER_SESSION_SKIP_OS_TESTS") == "1" {
		t.Skip("explicitly disabled: process namespace unavailable")
	}
	s := testStore(t)
	v := testCreate(t, s, "a", "persistent")
	lock, ok, e := s.runLock(v.ID)
	if e != nil || !ok {
		t.Fatal(e)
	}
	if e = s.Delete("a"); e == nil {
		t.Fatal("deleted a live session")
	}
	lock.Close()
	if e = s.Delete("a"); e != nil {
		t.Fatal(e)
	}
	temp := testCreate(t, s, "temporary-a", "temporary")
	if e = s.writeRun(temp.ID, Run{Phase: "reserved", Started: time.Now().Add(-time.Minute)}); e != nil {
		t.Fatal(e)
	}
	if notes := s.Cleanup(); len(notes) > 0 {
		t.Fatal(notes)
	}
	if _, e = s.Lookup(temp.ID); e == nil {
		t.Fatal("orphan temporary retained")
	}
}
func TestSymlinkRefusal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege is environment-dependent")
	}
	s := testStore(t)
	v := testCreate(t, s, "a", "persistent")
	dir, _ := s.Dir(v.ID)
	outside := t.TempDir()
	marker := filepath.Join(outside, "keep")
	os.WriteFile(marker, []byte("keep"), 0600)
	os.Remove(dir)
	if e := os.Symlink(outside, dir); e != nil {
		t.Fatal(e)
	}
	if e := s.Delete("a"); e == nil {
		t.Fatal("followed data symlink")
	}
	if _, e := os.Stat(marker); e != nil {
		t.Fatal("outside data damaged")
	}
}
func TestTemporaryReservation(t *testing.T) {
	s := testStore(t)
	v := testCreate(t, s, "temporary-a", "temporary")
	s.Cleanup()
	if _, e := s.Lookup(v.ID); e != nil {
		t.Fatal("cleanup raced temporary creation")
	}
}
