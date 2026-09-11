package platform

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSnapshotSelfAndBoot(t *testing.T) {
	if os.Getenv("BROWSER_SESSION_SKIP_OS_TESTS") == "1" {
		t.Skip("explicitly disabled: process namespace unavailable")
	}
	all, e := Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, p := range all {
		if p.PID == os.Getpid() {
			found = true
			if p.Birth == "" || len(p.Args) == 0 {
				t.Fatalf("incomplete self: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("self missing")
	}
	b, e := BootID()
	if e != nil || b == "" {
		t.Fatalf("boot: %q %v", b, e)
	}
}
func TestOwnershipAndPIDReuse(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a space")
	all := []Process{{PID: 10, Parent: 1, Birth: "new"}, {PID: 20, Parent: 1, Birth: "b", Args: []string{"chrome", "--user-data-dir=" + dir}}, {PID: 21, Parent: 20, Birth: "c"}, {PID: 22, Parent: 21, Birth: "d"}, {PID: 30, Parent: 1, Birth: "x", Args: []string{"chrome", "--user-data-dir=" + dir + "-other"}}}
	got := Owned(all, dir, []Process{{PID: 10, Birth: "old"}}, 0)
	var ids []int
	for _, p := range got {
		ids = append(ids, p.PID)
	}
	if !reflect.DeepEqual(ids, []int{20, 21, 22}) {
		t.Fatalf("wrong ownership: %v", ids)
	}
}
func TestExclusiveLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	l, ok, e := TryLock(p)
	if e != nil || !ok {
		t.Fatal(e)
	}
	_, ok, e = TryLock(p)
	if e != nil || ok {
		t.Fatal("second lock acquired", e)
	}
	l.Close()
	l, ok, e = TryLock(p)
	if e != nil || !ok {
		t.Fatal("lock not released", e)
	}
	l.Close()
}
func TestDataDirArguments(t *testing.T) {
	d := filepath.Join(t.TempDir(), "profile")
	for _, a := range [][]string{{"--user-data-dir=" + d}, {"--user-data-dir", d}, {"--database=" + filepath.Join(d, "Crashpad")}} {
		if !HasDataDir(a, d) {
			t.Fatal(a)
		}
	}
	if HasDataDir([]string{"--user-data-dir=" + d + "-other"}, d) {
		t.Fatal("prefix incorrectly matched")
	}
}

func TestUnreadableTrackedProcessBlocksCleanup(t *testing.T) {
	all := []Process{{PID: 10, Parent: 1, Name: "browser", Unreadable: true}, {PID: 11, Parent: 10, Name: "helper", Unreadable: true}, {PID: 99, Parent: 1, Name: "unrelated-system-service", Unreadable: true}}
	got := Owned(all, t.TempDir(), []Process{{PID: 10, Birth: "previous"}}, 0)
	if len(got) != 2 {
		t.Fatalf("tracked unreadable process/descendant must remain live: %+v", got)
	}
}
