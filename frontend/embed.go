//go:build desktop && !bindings

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS
var Assets fs.FS = func() fs.FS {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return assets
}()
