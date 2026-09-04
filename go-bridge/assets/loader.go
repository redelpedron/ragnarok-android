//go:build android

package assets

import (
	"io"
	"os"
	"path/filepath"
)

// defaultDataDir is a plain, top-level folder on shared storage — NOT under
// /sdcard/Android/data/<package>/, which most file managers can't reach on
// Android 11+ without root or a special toggle. Users just create this
// folder and drop data.grf (and rdata.grf) into it.
const defaultDataDir = "/storage/emulated/0/Ragnarok"

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
//  3. the default shared folder (/storage/emulated/0/Ragnarok)
//  4. the bare filename, relative to the process's working directory
func (al *AndroidAssetLoader) Open(name string) (io.ReadCloser, error) {
	if al.dataDir != "" {
		if f, err := os.Open(filepath.Join(al.dataDir, name)); err == nil {
			return f, nil
		}
	}
	if extDir := os.Getenv("GORO_DATA_DIR"); extDir != "" {
		if f, err := os.Open(filepath.Join(extDir, name)); err == nil {
			return f, nil
		}
	}
	if f, err := os.Open(filepath.Join(defaultDataDir, name)); err == nil {
		return f, nil
	}
	return os.Open(name)
}

func (al *AndroidAssetLoader) SetDataDir(path string) {
	al.dataDir = path
}
