// Package static provides the application's embedded web assets.
package static

import (
	"embed"
	"io/fs"
)

//go:embed css js templates
var assets embed.FS

// FS returns the embedded filesystem rooted at the static asset directory.
func FS() fs.FS { return assets }
