// Package platform contains operating system boundaries. No shell is used.
package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Process struct {
	PID    int      `json:"pid"`
	Parent int      `json:"-"`
	Group  int      `json:"-"`
	Birth  string   `json:"birth"`
	Args   []string `json:"-"`
}

func DataHome() (string, error) {
	h, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(h, "Library", "Application Support", "browser-session"), nil
	case "windows":
		d := os.Getenv("LOCALAPPDATA")
		if !filepath.IsAbs(d) {
			return "", errors.New("LOCALAPPDATA must be an absolute path")
		}
		return filepath.Join(d, "browser-session"), nil
	default:
		d := os.Getenv("XDG_DATA_HOME")
		if !filepath.IsAbs(d) {
			d = filepath.Join(h, ".local", "share")
		}
		return filepath.Join(d, "browser-session"), nil
	}
}

// Chromium may derive a cache outside user-data-dir. This path is derived, never trusted from JSON.
func CacheDir(data string) string {
	h, _ := os.UserHomeDir()
	var config, cache string
	switch runtime.GOOS {
	case "darwin":
		config = filepath.Join(h, "Library", "Application Support")
		cache = filepath.Join(h, "Library", "Caches")
	case "linux":
		config = os.Getenv("XDG_CONFIG_HOME")
		if !filepath.IsAbs(config) {
			config = filepath.Join(h, ".config")
		}
		cache = os.Getenv("XDG_CACHE_HOME")
		if !filepath.IsAbs(cache) {
			cache = filepath.Join(h, ".cache")
		}
	default:
		return data
	}
	rel, e := filepath.Rel(config, data)
	if e == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join(cache, rel)
	}
	return data
}

func SamePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func HasDataDir(args []string, dir string) bool {
	for i, a := range args {
		if strings.HasPrefix(a, "--user-data-dir=") && SamePath(strings.TrimPrefix(a, "--user-data-dir="), dir) {
			return true
		}
		if a == "--user-data-dir" && i+1 < len(args) && SamePath(args[i+1], dir) {
			return true
		}
		// Crashpad can outlive the browser and detach from its process group.
		if strings.HasPrefix(a, "--database=") {
			d := strings.TrimPrefix(a, "--database=")
			rel, e := filepath.Rel(dir, d)
			if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

func Owned(all []Process, dir string, known []Process, group int) []Process {
	ids := map[int]bool{}
	for _, p := range all {
		if HasDataDir(p.Args, dir) || group > 0 && p.Group == group {
			ids[p.PID] = true
		}
		for _, old := range known {
			if old.PID == p.PID && old.Birth == p.Birth {
				ids[p.PID] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, p := range all {
			if ids[p.Parent] && !ids[p.PID] {
				ids[p.PID] = true
				changed = true
			}
		}
	}
	var result []Process
	for _, p := range all {
		if ids[p.PID] {
			p.Args = nil
			result = append(result, p)
		}
	}
	return result
}

// Snapshot fails closed if /proc or the native API describes a different PID namespace.
func Snapshot() ([]Process, error) {
	all, e := snapshot()
	if e != nil {
		return nil, e
	}
	for _, p := range all {
		if p.PID == os.Getpid() && len(p.Args) > 0 && p.Args[0] == os.Args[0] {
			return all, nil
		}
	}
	return nil, fmt.Errorf("process inspection is incomplete or PID namespace does not match; data retained")
}
