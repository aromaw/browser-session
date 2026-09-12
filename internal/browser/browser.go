// Package browser discovers and launches the user's existing browser.
package browser

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aromaw/browser-session/internal/platform"
)

type Browser struct {
	ID         string `json:"id"`
	Executable string `json:"executable"`
}
type Launcher struct{}

func (l Launcher) DetectBrowsers() []Browser {
	h, _ := os.UserHomeDir()
	specs := []struct {
		id               string
		names, apps, win []string
	}{
		{"chrome", []string{"google-chrome", "google-chrome-stable", "chrome"}, []string{"Google Chrome.app"}, []string{"Google/Chrome/Application/chrome.exe"}},
		{"chromium", []string{"chromium", "chromium-browser"}, []string{"Chromium.app"}, []string{"Chromium/Application/chrome.exe", "Chromium/chrome.exe"}},
	}
	var out []Browser
	seen := map[string]bool{}
	for _, s := range specs {
		var candidates []string
		switch runtime.GOOS {
		case "darwin":
			for _, base := range []string{"/Applications", filepath.Join(h, "Applications")} {
				for _, a := range s.apps {
					candidates = append(candidates, filepath.Join(base, a))
				}
			}
		case "windows":
			for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
				if !filepath.IsAbs(base) {
					continue
				}
				for _, p := range s.win {
					candidates = append(candidates, filepath.Join(base, filepath.FromSlash(p)))
				}
			}
		}
		candidates = append(candidates, s.names...)
		for _, p := range candidates {
			v, e := resolvePath(p)
			if e == nil && !seen[v] {
				seen[v] = true
				out = append(out, Browser{s.id, v})
			}
		}
	}
	return out
}
func (l Launcher) ResolveExecutable(choice string) (Browser, error) {
	if choice == "" || choice == "auto" || choice == "chrome" || choice == "chromium" {
		for _, b := range l.DetectBrowsers() {
			if choice == "" || choice == "auto" || choice == b.ID {
				return b, nil
			}
		}
		return Browser{}, fmt.Errorf("no %s browser found; use --browser with a Chrome/Chromium executable path", choice)
	}
	p, e := resolvePath(choice)
	if e != nil {
		return Browser{}, e
	}
	return Browser{"custom", p}, nil
}
func resolvePath(path string) (string, error) {
	if runtime.GOOS == "darwin" && strings.HasSuffix(strings.ToLower(path), ".app") {
		f, e := os.Open(filepath.Join(path, "Contents", "Info.plist"))
		if e != nil {
			return "", e
		}
		defer f.Close()
		dec := xml.NewDecoder(f)
		key := ""
		exe := ""
		for {
			t, e := dec.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return "", errors.New("cannot read app bundle; specify its Contents/MacOS executable")
			}
			if s, ok := t.(xml.StartElement); ok {
				if s.Name.Local == "key" {
					_ = dec.DecodeElement(&key, &s)
				} else if s.Name.Local == "string" && key == "CFBundleExecutable" {
					_ = dec.DecodeElement(&exe, &s)
					break
				}
			}
		}
		if exe == "" || filepath.Base(exe) != exe {
			return "", errors.New("invalid CFBundleExecutable")
		}
		path = filepath.Join(path, "Contents", "MacOS", exe)
	}
	p, e := exec.LookPath(path)
	if e != nil {
		return "", fmt.Errorf("browser executable unavailable: %w", e)
	}
	p, e = filepath.Abs(p)
	if e != nil {
		return "", e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return "", e
	}
	st, e := os.Stat(p)
	if e != nil {
		return "", e
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("browser must be a regular executable file")
	}
	if runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(p), ".exe") {
		return "", errors.New("Windows requires an .exe, not a shell script")
	}
	if runtime.GOOS == "linux" && (strings.HasPrefix(p, "/snap/") || strings.Contains(p, "/flatpak/")) {
		return "", errors.New("Snap/Flatpak browsers are not supported in this MVP; use a native install")
	}
	return p, nil
}
func ValidateURLs(urls []string) error {
	for _, raw := range urls {
		u, e := url.Parse(raw)
		if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			return fmt.Errorf("only absolute http/https URLs without credentials are accepted")
		}
	}
	return nil
}
func (l Launcher) BuildArgs(dir string, urls []string) ([]string, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("user-data-dir must be absolute")
	}
	if e := ValidateURLs(urls); e != nil {
		return nil, e
	}
	args := []string{"--user-data-dir=" + dir, "--disk-cache-dir=" + filepath.Join(dir, "Cache"), "--no-first-run", "--no-default-browser-check", "--disable-background-mode", "--disable-sync", "--new-window"}
	if len(urls) == 0 {
		args = append(args, "about:blank")
	} else {
		args = append(args, urls...)
	}
	return args, nil
}
func (l Launcher) Launch(exe, dir string, urls []string) (*exec.Cmd, error) {
	args, e := l.BuildArgs(dir, urls)
	if e != nil {
		return nil, e
	}
	c := exec.Command(exe, args...)
	platform.PrepareBrowser(c)
	// Browser output is intentionally discarded: it may include URLs or credentials.
	if e = c.Start(); e != nil {
		return nil, e
	}
	return c, nil
}
func (l Launcher) IsRunning(exe, dir string, known []platform.Process, group int) ([]platform.Process, error) {
	all, e := platform.Snapshot()
	if e != nil {
		return nil, e
	}
	return l.InspectSnapshot(all, exe, dir, known, group)
}

// InspectSnapshot applies the same fail-closed checks to a shared snapshot.
func (l Launcher) InspectSnapshot(all []platform.Process, exe, dir string, known []platform.Process, group int) ([]platform.Process, error) {
	for _, p := range all {
		if p.Unreadable && strings.EqualFold(p.Name, filepath.Base(exe)) {
			return nil, fmt.Errorf("cannot inspect browser PID %d; data retained", p.PID)
		}
	}
	return platform.Owned(all, dir, known, group), nil
}
