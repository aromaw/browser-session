package native

import (
	"encoding/json"
	"errors"
	"github.com/aromaw/browser-session/internal/browser"
	"github.com/aromaw/browser-session/internal/platform"
	"github.com/aromaw/browser-session/internal/session"
	"os"
	"path/filepath"
	"runtime"
)

type Manifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func ManifestPath(goos, home, xdg, chromeRoot, appRoot string) string {
	if goos == "windows" {
		return filepath.Join(appRoot, HostName+".json")
	}
	if chromeRoot == "" {
		if goos == "darwin" {
			chromeRoot = filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
		} else {
			if !filepath.IsAbs(xdg) {
				xdg = filepath.Join(home, ".config")
			}
			chromeRoot = filepath.Join(xdg, "google-chrome")
		}
	}
	return filepath.Join(chromeRoot, "NativeMessagingHosts", HostName+".json")
}
func Install(id, choice, root, chromeRoot string) (string, error) {
	if !extensionID.MatchString(id) {
		return "", errors.New("extension ID must be 32 letters a-p")
	}
	if chromeRoot != "" && !filepath.IsAbs(chromeRoot) {
		return "", errors.New("Chrome data directory must be absolute")
	}
	b, err := (browser.Launcher{}).ResolveExecutable(choice)
	if err != nil {
		return "", err
	}
	s, err := session.NewStore(root)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	appRoot, err := platform.DataHome()
	if err != nil {
		return "", err
	}
	// Also secure the default app root if the caller chose another session root.
	if _, err = session.NewStore(appRoot); err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	origin := "chrome-extension://" + id + "/"
	config, err := configPath()
	if err != nil {
		return "", err
	}
	if err = writeJSON(config, Config{Origin: origin, Root: s.Root, Browser: b.Executable}); err != nil {
		return "", err
	}
	path := ManifestPath(runtime.GOOS, home, os.Getenv("XDG_CONFIG_HOME"), chromeRoot, appRoot)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if err = writeJSON(path, Manifest{Name: HostName, Description: "Open a new isolated temporary Chrome session", Path: exe, Type: "stdio", AllowedOrigins: []string{origin}}); err != nil {
		return "", err
	}
	if err = register(path); err != nil {
		return "", err
	}
	return path, nil
}
func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".native-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	return platform.Replace(f.Name(), path)
}
