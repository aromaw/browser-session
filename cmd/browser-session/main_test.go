package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aromaw/browser-session/internal/platform"
	"github.com/aromaw/browser-session/internal/session"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "__supervise" {
		if e := run(os.Args[1:], os.Stdout, os.Stderr); e != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "__fake-child" {
		for !exists(filepath.Join(os.Args[2], "child-stop")) {
			time.Sleep(40 * time.Millisecond)
		}
		os.Exit(0)
	}
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--user-data-dir=") {
		fakeBrowser(strings.TrimPrefix(os.Args[1], "--user-data-dir="))
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func exists(p string) bool { _, e := os.Stat(p); return e == nil }
func fakeBrowser(dir string) {
	// A compiled test fixture; never opens websites or real user profiles.
	os.WriteFile(filepath.Join(dir, "synthetic-account"), []byte("test account"), 0600)
	exe, _ := os.Executable()
	child := exec.Command(exe, "__fake-child", dir)
	if child.Start() != nil {
		os.Exit(2)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	closed := fakeWindow()
	for {
		select {
		case <-sig:
			goto end
		case <-closed:
			goto end
		default:
		}
		if exists(filepath.Join(dir, "stop")) {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
end:
	if !exists(filepath.Join(dir, "keep-child")) {
		os.WriteFile(filepath.Join(dir, "child-stop"), []byte("stop"), 0600)
		child.Wait()
	}
}
func fixture(t *testing.T) *session.Store {
	t.Helper()
	if os.Getenv("BROWSER_SESSION_SKIP_OS_TESTS") == "1" {
		t.Skip("explicitly disabled: process namespace unavailable")
	}
	if _, e := platform.Snapshot(); e != nil {
		t.Fatal(e)
	}
	s, e := session.NewStore(filepath.Join(t.TempDir(), "data with spaces 中文"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c, _ := s.Read()
		for _, v := range c.Sessions {
			d := s.DataPath(v)
			os.WriteFile(filepath.Join(d, "stop"), nil, 0600)
			os.WriteFile(filepath.Join(d, "child-stop"), nil, 0600)
		}
		time.Sleep(1500 * time.Millisecond)
	})
	return s
}
func newFake(t *testing.T, s *session.Store, name, kind string) session.Session {
	t.Helper()
	exe, _ := os.Executable()
	v, e := s.Create(name, kind, exe)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
func TestPersistentLifecycleAndIsolation(t *testing.T) {
	s := fixture(t)
	a := newFake(t, s, "a", "persistent")
	b := newFake(t, s, "b", "persistent")
	if _, e := s.Open("a", nil); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Open("b", nil); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Open("a", nil); e == nil {
		t.Fatal("duplicate open accepted")
	}
	if e := s.Delete("a"); e == nil {
		t.Fatal("active deletion accepted")
	}
	if e := s.Close("a"); e != nil {
		t.Fatal(e)
	}
	if s.Status(b) != "running" {
		t.Fatal("B affected by closing A")
	}
	if !exists(filepath.Join(s.DataPath(a), "synthetic-account")) {
		t.Fatal("persistent storage lost")
	}
	os.Remove(filepath.Join(s.DataPath(a), "child-stop"))
	if _, e := s.Open("a", nil); e != nil {
		t.Fatal(e)
	}
	if e := s.Close("a"); e != nil {
		t.Fatal(e)
	}
	if e := s.Close("b"); e != nil {
		t.Fatal(e)
	}
}
func TestTemporaryWaitsForChildren(t *testing.T) {
	s := fixture(t)
	a := newFake(t, s, "temporary-a", "temporary")
	b := newFake(t, s, "temporary-b", "temporary")
	if _, e := s.Open(a.Name, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Open(b.Name, nil); e != nil {
		t.Fatal(e)
	}
	d := s.DataPath(a)
	os.WriteFile(filepath.Join(d, "keep-child"), nil, 0600)
	os.WriteFile(filepath.Join(d, "stop"), nil, 0600)
	time.Sleep(1600 * time.Millisecond)
	if !exists(d) {
		t.Fatal("temporary removed before child exit")
	}
	if s.Status(b) != "running" {
		t.Fatal("temporary B affected")
	}
	os.WriteFile(filepath.Join(d, "child-stop"), nil, 0600)
	eventually(t, func() bool { return !exists(d) })
	if e := s.Close(b.Name); e != nil {
		t.Fatal(e)
	}
	if exists(s.DataPath(b)) {
		t.Fatal("temporary B data retained")
	}
}
func TestSupervisorCrashRecovery(t *testing.T) {
	s := fixture(t)
	a := newFake(t, s, "temporary-a", "temporary")
	if _, e := s.Open(a.Name, nil); e != nil {
		t.Fatal(e)
	}
	all, e := platform.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	killed := false
	for _, p := range all {
		if len(p.Args) >= 5 && p.Args[1] == "__supervise" && p.Args[3] == a.ID {
			proc, e := os.FindProcess(p.PID)
			if e != nil {
				t.Fatal(e)
			}
			if e = proc.Kill(); e != nil {
				t.Fatal(e)
			}
			killed = true
		}
	}
	if !killed {
		t.Fatal("supervisor missing")
	}
	time.Sleep(300 * time.Millisecond)
	s.Cleanup()
	d := s.DataPath(a)
	if !exists(d) {
		t.Fatal("cleanup deleted live orphan")
	}
	os.WriteFile(filepath.Join(d, "stop"), nil, 0600)
	eventually(t, func() bool { s.Cleanup(); return !exists(d) })
}
func TestCLIRejectsUnsafeInput(t *testing.T) {
	var out, errOut bytes.Buffer
	if e := run([]string{"--data-dir", t.TempDir(), "delete", "anything"}, &out, &errOut); e == nil {
		t.Fatal("deletion without --yes accepted")
	}
	if e := run([]string{"help"}, &out, &errOut); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "browser-session open") {
		t.Fatal(fmt.Sprint(out.String()))
	}
}
