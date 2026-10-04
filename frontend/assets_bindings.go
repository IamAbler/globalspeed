//go:build bindings

package frontend

import "io/fs"

// Wails generates Go bindings before building the frontend. Its bindings
// runner does not serve assets, so it must not require a prebuilt dist folder.
var Assets fs.FS
