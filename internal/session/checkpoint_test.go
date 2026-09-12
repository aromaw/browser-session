package session

import (
	"errors"
	"github.com/aromaw/browser-session/internal/platform"
	"testing"
)

func TestCheckpointPersistsIdentityChangesWithoutHeartbeats(t *testing.T) {
	writes := 0
	fail := false
	c := runCheckpoint{write: func(Run) error {
		writes++
		if fail {
			return errors.New("disk full")
		}
		return nil
	}}
	r := Run{Phase: "running", Processes: []platform.Process{{PID: 2, Birth: "a"}, {PID: 1, Birth: "b"}}}
	if err := c.Save(r); err != nil {
		t.Fatal(err)
	}
	r.Processes[0], r.Processes[1] = r.Processes[1], r.Processes[0]
	for i := 0; i < 100; i++ {
		if err := c.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 {
		t.Fatalf("unchanged snapshots wrote %d times", writes)
	}
	r.Processes[0].Birth = "reused PID"
	fail = true
	if c.Save(r) == nil {
		t.Fatal("write failure hidden")
	}
	fail = false
	if err := c.Save(r); err != nil {
		t.Fatal(err)
	}
	if writes != 3 {
		t.Fatal("failed change was not retried")
	}
}

func TestSupervisorDispatchRejectsMalformedInvocation(t *testing.T) {
	if ok, err := DispatchSupervisor([]string{"__supervise"}); !ok || err == nil {
		t.Fatal(ok, err)
	}
	if ok, err := DispatchSupervisor([]string{"open"}); ok || err != nil {
		t.Fatal(ok, err)
	}
}
