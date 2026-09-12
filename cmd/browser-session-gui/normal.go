//go:build gui && !guitest

package main

import "github.com/wailsapp/wails/v2/pkg/options"

func prepareGUI(root string) (string, func()) { return root, func() {} }
func configureSmoke(*options.App)             {}
