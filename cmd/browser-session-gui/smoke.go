//go:build gui && guitest

// This harness is compiled only into CI's separate smoke binary, never releases.
package main

import (
	"bytes"
	"context"
	"embed"
	"io/fs"
	"os"
	"sync"
	"testing/fstest"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed smoke.js
var smokeJS embed.FS

type Smoke struct {
	ctx  context.Context
	mu   sync.Mutex
	done bool
}

func (*Smoke) Executable() string { exe, _ := os.Executable(); return exe }
func (s *Smoke) Finish(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}
	s.done = true
	_ = os.WriteFile(os.Getenv("BROWSER_SESSION_GUI_REPORT"), []byte(message), 0600)
	wruntime.Quit(s.ctx)
}
func prepareGUI(string) (string, func()) {
	// Always use a newly allocated directory, never a user's existing store.
	root, err := os.MkdirTemp("", "browser-session-ui-smoke-")
	if err != nil {
		panic(err)
	}
	return root, func() { _ = os.RemoveAll(root) }
}
func configureSmoke(o *options.App) {
	assets := fstest.MapFS{}
	_ = fs.WalkDir(o.AssetServer.Assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(o.AssetServer.Assets, path)
		if err != nil {
			return err
		}
		if path == "index.html" {
			b = bytes.Replace(b, []byte("</body>"), []byte(`<script src="smoke.js"></script></body>`), 1)
		}
		assets[path] = &fstest.MapFile{Data: b, Mode: 0444}
		return nil
	})
	js, _ := smokeJS.ReadFile("smoke.js")
	assets["smoke.js"] = &fstest.MapFile{Data: js, Mode: 0444}
	o.AssetServer.Assets = assets
	smoke := &Smoke{}
	o.Bind = append(o.Bind, smoke)
	startup := o.OnStartup
	o.OnStartup = func(ctx context.Context) { startup(ctx); smoke.ctx = ctx }
	// A stuck native engine must fail CI instead of leaving a hanging job.
	go func() { time.Sleep(70 * time.Second); os.Exit(3) }()
}
