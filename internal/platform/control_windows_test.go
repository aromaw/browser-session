package platform

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestControlChild(t *testing.T) {
	if os.Getenv("BROWSER_SESSION_CONTROL_CHILD") == "1" {
		os.Exit(0)
	}
}

func TestBrowserControlRetainsExitedProcessIdentity(t *testing.T) {
	if os.Getenv("BROWSER_SESSION_SKIP_OS_TESTS") == "1" {
		t.Skip("native process API unavailable")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestControlChild$")
	cmd.Env = append(os.Environ(), "BROWSER_SESSION_CONTROL_CHILD=1")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	control, err := CaptureBrowser(cmd.Process)
	if err != nil {
		_ = cmd.Wait()
		t.Fatal(err)
	}
	defer control.Release()
	if err = cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	pid, err := windows.GetProcessId(control.h)
	if err != nil || int(pid) != cmd.Process.Pid {
		t.Fatal("original identity not retained", pid, err)
	}
	if err := control.CloseWindow(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatal("exited process accepted window close", err)
	}
}
