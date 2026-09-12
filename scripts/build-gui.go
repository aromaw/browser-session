//go:build ignore

// Build the native GUI without a frontend package manager or the Wails CLI.
// Run from the repository root: go run ./scripts/build-gui.go
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	target := flag.String("target", runtime.GOOS+"-"+runtime.GOARCH, "OS-architecture")
	version := flag.String("version", "dev", "version")
	smoke := flag.Bool("smoke", false, "build test-only native UI harness")
	flag.Parse()
	if err := build(*target, *version, *smoke); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func build(target, version string, smoke bool) error {
	parts := strings.Split(target, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid target")
	}
	goos, arch := parts[0], parts[1]
	if (goos != "darwin" && goos != "linux" && goos != "windows") || (arch != "amd64" && arch != "arm64") {
		return fmt.Errorf("unsupported target")
	}
	if strings.ContainsAny(version, " \t\r\n/\\") {
		return fmt.Errorf("invalid version")
	}
	stage := filepath.Join("dist", "gui-"+target)
	name := "browser-session-gui"
	if goos == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(stage, name)
	if goos == "darwin" {
		binary = filepath.Join(stage, "Browser Sessions.app", "Contents", "MacOS", name)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		return err
	}
	tags := "gui,desktop,production"
	if goos == "linux" {
		tags += ",webkit2_41"
	}
	if smoke {
		tags += ",guitest"
	}
	ldflags := "-s -w -X main.version=" + version
	if goos == "windows" {
		ldflags += " -H windowsgui"
	}
	cmd := exec.Command("go", "build", "-trimpath", "-tags", tags, "-ldflags", ldflags, "-o", binary, "./cmd/browser-session-gui")
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+arch)
	if goos == "darwin" {
		cmd.Env = append(cmd.Env, "MACOSX_DEPLOYMENT_TARGET=13.0")
	}
	if goos == "windows" {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if goos == "darwin" {
		info := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>browser-session-gui</string><key>CFBundleIdentifier</key><string>io.github.aromaw.browser-session</string><key>CFBundleName</key><string>Browser Sessions</string><key>CFBundleDisplayName</key><string>Browser Sessions</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleShortVersionString</key><string>0.2.0</string><key>CFBundleVersion</key><string>2</string><key>LSMinimumSystemVersion</key><string>13.0</string><key>NSHighResolutionCapable</key><true/></dict></plist>`
		if err := os.WriteFile(filepath.Join(stage, "Browser Sessions.app", "Contents", "Info.plist"), []byte(info), 0644); err != nil {
			return err
		}
		// Ad-hoc signing makes the bundle internally consistent on Apple Silicon;
		// it is not Developer ID signing or notarization.
		if runtime.GOOS == "darwin" {
			sign := exec.Command("codesign", "--force", "--deep", "--sign", "-", filepath.Join(stage, "Browser Sessions.app"))
			sign.Stdout = os.Stdout
			sign.Stderr = os.Stderr
			if err := sign.Run(); err != nil {
				return err
			}
		}
	}
	for _, file := range []string{"README.md", "LICENSE"} {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(stage, file), data, 0644); err != nil {
			return err
		}
	}
	if goos == "linux" {
		desktop := `[Desktop Entry]
Type=Application
Name=Browser Sessions
Comment=Independent Chrome / Chromium sessions
Exec=browser-session-gui
Icon=browser-session
Terminal=false
Categories=Network;Utility;
`
		if err := os.WriteFile(filepath.Join(stage, "browser-session.desktop"), []byte(desktop), 0644); err != nil {
			return err
		}
		data, err := os.ReadFile("internal/desktop/web/icon.svg")
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(stage, "browser-session.svg"), data, 0644); err != nil {
			return err
		}
	}
	if smoke {
		fmt.Println(binary)
		return nil
	}
	archive := filepath.Join("dist", "browser-session-gui-"+version+"-"+target+".zip")
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(stage, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		dest, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()
		_, err = io.Copy(dest, source)
		return err
	})
	if err != nil {
		_ = zw.Close()
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	fmt.Println(archive)
	return nil
}
