//go:build android

package assets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDataDir is a plain, top-level folder on shared storage — NOT under
// /sdcard/Android/data/<package>/, which most file managers can't reach on
// Android 11+ without root or a special toggle. Users just create this
// folder and drop data.grf (and rdata.grf) into it.
const DefaultDataDir = "/storage/emulated/11/Ragnarok"

// AndroidAssetLoader resolves RO data files on Android.
type AndroidAssetLoader struct {
	dataDir string
}

func New(dataDir string) *AndroidAssetLoader {
	return &AndroidAssetLoader{dataDir: dataDir}
}

// Open tries, in order:
//  1. an explicitly-set dataDir (e.g. from a SAF folder picker, if wired up)
//  2. the GORO_DATA_DIR environment variable
//  3. DefaultDataDir (/storage/emulated/11/Ragnarok)
//  4. the bare filename, relative to the process's working directory
//
// If every candidate fails, the returned error lists each path tried and
// its specific error (missing file vs permission denied vs something else)
// instead of just the last one, so a single logcat line is enough to debug.
func (al *AndroidAssetLoader) Open(name string) (io.ReadCloser, error) {
	var candidates []string
	if al.dataDir != "" {
		candidates = append(candidates, filepath.Join(al.dataDir, name))
	}
	if extDir := os.Getenv("GORO_DATA_DIR"); extDir != "" {
		candidates = append(candidates, filepath.Join(extDir, name))
	}
	candidates = append(candidates, filepath.Join(DefaultDataDir, name))
	candidates = append(candidates, name)

	var attempts []string
	for _, p := range candidates {
		f, err := os.Open(p)
		if err == nil {
			return f, nil
		}
		attempts = append(attempts, fmt.Sprintf("%s (%v)", p, err))
	}
	return nil, fmt.Errorf("%s not found, tried: %s", name, strings.Join(attempts, "; "))
}

func (al *AndroidAssetLoader) SetDataDir(path string) {
	al.dataDir = path
}

// ResolvePath returns the first candidate path where name exists, using the
// same search order as Open, without opening the file. Useful for callers
// like res.OpenGRF that want a path rather than an io.ReadCloser.
func (al *AndroidAssetLoader) ResolvePath(name string) (string, error) {
	var candidates []string
	if al.dataDir != "" {
		candidates = append(candidates, filepath.Join(al.dataDir, name))
	}
	if extDir := os.Getenv("GORO_DATA_DIR"); extDir != "" {
		candidates = append(candidates, filepath.Join(extDir, name))
	}
	candidates = append(candidates, filepath.Join(DefaultDataDir, name))
	candidates = append(candidates, name)

	var attempts []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		} else {
			attempts = append(attempts, fmt.Sprintf("%s (%v)", p, err))
		}
	}
	return "", fmt.Errorf("%s not found, tried: %s", name, strings.Join(attempts, "; "))
}
