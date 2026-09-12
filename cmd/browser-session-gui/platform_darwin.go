//go:build gui

package main

// Wails' native file dialog uses UTType on current SDKs. Link the corresponding
// system framework explicitly when building directly with Go, without its CLI.

// #cgo LDFLAGS: -framework UniformTypeIdentifiers
import "C"
