package httpsrv

import (
	"embed"
	"io/fs"
)

//go:embed ui/*
var uiAssets embed.FS

// UIFS returns the embedded UI assets rooted at ui/, ready to pass to
// Server.WithUI. Returns nil if go:embed somehow failed (it can't, but the
// signature lets callers fall back gracefully).
func UIFS() fs.FS {
	sub, err := fs.Sub(uiAssets, "ui")
	if err != nil {
		return nil
	}
	return sub
}
