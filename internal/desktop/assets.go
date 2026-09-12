package desktop

import (
	"embed"
	"io/fs"
)

// Only the UI is embedded, never the profile root or development fixtures.
//
//go:embed web/index.html web/app.js web/model.js web/style.css web/icon.svg
var files embed.FS

func Assets() fs.FS {
	assets, err := fs.Sub(files, "web")
	if err != nil {
		panic(err)
	}
	return assets
}
