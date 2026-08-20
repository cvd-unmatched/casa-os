//go:build windows

package service

import (
	"archive/tar"
	"os"
)

// This binary only ever ships for Linux (see .github/workflows/release.yml,
// GOOS=linux on every build) - these are no-op stubs that exist purely so
// this package builds and its OS-independent logic is testable on a
// Windows dev machine. See backup_platform_unix.go for the real behavior.
func setTarOwnership(header *tar.Header, info os.FileInfo) {}

func checkFreeSpace(dir string, needed int64) error { return nil }
