package httpfs

import (
	"fmt"
)

var (
	// Package package name
	Package = "httpfs"

	// Version release version
	Version = "0.0.1"

	// Commit will be overwritten automatically by the build system
	Commit = "HEAD"
)

// FullVersion display the full version and build
func FullVersion() string {
	return fmt.Sprintf("%s-%s@%s", Package, Version, Commit)
}
