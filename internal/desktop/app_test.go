package desktop

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/aromaw/browser-session/internal/session"
)

func TestDesktopAndCLIShareStoreAndRejectUnsafeInputs(t *testing.T) {
	root := t.TempDir()
	app := New(root, "test", nil)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateSession("personal", "persistent", exe, "javascript:alert(1)", false); err == nil {
		t.Fatal("unsafe URL accepted")
	}
	config, err := app.store.Read()
	if err != nil || len(config.Sessions) != 0 {
		t.Fatal("invalid input created data", err)
	}
	created, err := app.CreateSession("personal", "persistent", exe, "", false)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := session.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	found, err := cli.Lookup("personal")
	if err != nil || found.ID != created.ID {
		t.Fatal("CLI cannot see GUI session", err)
	}
	if err := app.DeleteSession(created.ID, "work"); err == nil {
		t.Fatal("deletion confirmation bypassed")
	}
	if _, err := cli.Lookup(created.ID); err != nil {
		t.Fatal("unconfirmed delete mutated data")
	}
	state, err := app.Snapshot()
	if err != nil || len(state.Rows) != 1 || state.Rows[0].DataDir != cli.DataPath(found) {
		t.Fatal(state, err)
	}
	if state.Version != "test" {
		t.Fatal("missing version")
	}
}

func TestPickerCancellationAndInitializationFailure(t *testing.T) {
	app := New(t.TempDir(), "test", func() (string, error) { return "", nil })
	if value, err := app.PickBrowser(); value != "" || err != nil {
		t.Fatal("cancel should be harmless", value, err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	app = New(file, "test", nil)
	if _, err := app.Snapshot(); err == nil {
		t.Fatal("initialization failure hidden")
	}
	if err := app.OpenSession("anything", ""); err == nil {
		t.Fatal("operation ignored initialization failure")
	}
}

func TestAssetBoundaryContainsOnlyUI(t *testing.T) {
	assets := Assets()
	entries, err := fs.ReadDir(assets, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("unexpected embedded files: %v", entries)
	}
	for _, path := range []string{"../sessions.json", "sessions.json", "test-harness.js", "browser-data/Cookies"} {
		if _, err := fs.ReadFile(assets, path); err == nil {
			t.Fatalf("exposed %s", path)
		}
	}
}
