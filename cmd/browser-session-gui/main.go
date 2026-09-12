//go:build gui

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/aromaw/browser-session/internal/desktop"
	"github.com/aromaw/browser-session/internal/session"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var version = "dev"

func main() {
	// This branch never creates a WebView, even when the supervisor executable
	// lives inside the macOS .app bundle or is a Windows GUI subsystem binary.
	if handled, err := session.DispatchSupervisor(os.Args[1:]); handled {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	root := flag.String("data-dir", os.Getenv("BROWSER_SESSION_HOME"), "session data directory (shared with CLI)")
	flag.Parse()
	resolvedRoot, finish := prepareGUI(*root)
	defer finish()
	var ctx context.Context
	var ctxMu sync.RWMutex
	app := desktop.New(resolvedRoot, version, func() (string, error) {
		ctxMu.RLock()
		current := ctx
		ctxMu.RUnlock()
		if current == nil {
			return "", fmt.Errorf("界面尚未就绪。")
		}
		return wruntime.OpenFileDialog(current, wruntime.OpenDialogOptions{Title: "选择 Chrome / Chromium 浏览器", ResolvesAliases: true})
	})
	// UI storage is separate from every managed browser profile. Wails' WebView
	// loads embedded local assets only; no remote page is navigated here.
	cache, err := os.UserCacheDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	settings := &options.App{
		Title: "Browser Sessions", Width: 1120, Height: 760, MinWidth: 760, MinHeight: 540,
		BackgroundColour: options.NewRGB(247, 248, 245),
		AssetServer:      &assetserver.Options{Assets: desktop.Assets()},
		OnStartup:        func(c context.Context) { ctxMu.Lock(); ctx = c; ctxMu.Unlock() },
		Bind:             []interface{}{app},
		Windows:          &windows.Options{WebviewUserDataPath: filepath.Join(cache, "browser-session-ui")},
		DragAndDrop:      &options.DragAndDrop{DisableWebViewDrop: true},
	}
	configureSmoke(settings)
	err = wails.Run(settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
