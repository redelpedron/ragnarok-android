//go:build android

// Package assets resolves the on-device directory holding the RO client's
// GRF archives.
//
// Earlier version of this file re-implemented file opening itself
// (AndroidAssetLoader.Open, returning an io.ReadCloser) as a stand-in for
// "does goro's asset loading work". That was never actually wired to goro -
// it was a parallel path that only proved a single named file could be
// os.Open'd.
//
// The real upstream API, github.com/kivutar/goro/res, doesn't want a file
// handle at all: res.NewManager(dir) takes a *directory* and does its own
// scan for data.grf/rdata.grf/fdata.grf/event.grf plus any other *.grf/*.gpf
// file it finds there (see res/manager.go's scanKnownFiles), opening each
// with the real GRF parser. So the only job left for this package is
// figuring out *which directory* to hand to res.NewManager - Android has no
// single obvious place for user-supplied game data the way a desktop client
// has "next to the .exe".
package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDataDir is a plain, top-level folder on shared storage — NOT under
// /sdcard/Android/data/<package>/, which most file managers can't reach on
// Android 11+ without root or a special toggle. Users just create this
// folder and drop data.grf (and rdata.grf) into it.
const DefaultDataDir = "/storage/emulated/0/Ragnarok"

// knownArchives mirrors the filenames res.Manager.scanKnownFiles() looks
// for, so "this directory has game data" means the same thing here as it
// does inside goro itself.
var knownArchives = []string{"data.grf", "rdata.grf", "fdata.grf", "event.grf"}

// ResolveDataDir finds the directory that holds the client's GRF archives,
// trying in order:
//  1. explicit (e.g. from a SAF folder picker, if wired up later)
//  2. the GORO_DATA_DIR environment variable
//  3. DefaultDataDir (/storage/emulated/0/Ragnarok)
//  4. the process's current working directory
//
// A candidate only counts if it actually contains a .grf/.gpf file — not
// merely if the directory exists — because res.NewManager happily builds a
// Manager for an empty directory and only fails much later, deep inside the
// engine, in a way that's hard to trace back to "wrong folder" from a
// phone's logcat. Resolving that here, once, up front, keeps the failure
// message actionable.
//
// If every candidate fails, the returned error lists each one tried and why,
// so a single logcat line is enough to debug.
func ResolveDataDir(explicit string) (string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if extDir := os.Getenv("GORO_DATA_DIR"); extDir != "" {
		candidates = append(candidates, extDir)
	}
	candidates = append(candidates, DefaultDataDir)
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}

	var attempts []string
	for _, dir := range candidates {
		ok, reason := hasArchive(dir)
		if ok {
			return dir, nil
		}
		attempts = append(attempts, fmt.Sprintf("%s (%s)", dir, reason))
	}
	return "", fmt.Errorf("no .grf/.gpf archive found, tried: %s", strings.Join(attempts, "; "))
}

// hasArchive reports whether dir contains at least one file res.NewManager
// would actually load: any of the well-known names, or any other *.grf/*.gpf
// file (case-insensitive, matching res.Manager's own scan).
func hasArchive(dir string) (bool, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err.Error()
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".grf") || strings.HasSuffix(name, ".gpf") {
			return true, ""
		}
	}
	for _, name := range knownArchives {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true, ""
		}
	}
	return false, "no .grf/.gpf files"
}
