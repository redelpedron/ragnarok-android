//go:build android

package assets

import (
	"io"
	"os"
	"path/filepath"
)

// AndroidAssetLoader resolves RO data files on Android.
type AndroidAssetLoader struct {
	dataDir string
}

func New(dataDir string) *AndroidAssetLoader {
	return &AndroidAssetLoader{dataDir: dataDir}
}

func (al *AndroidAssetLoader) Open(name string) (io.ReadCloser, error) {
	if al.dataDir != "" {
		p := filepath.Join(al.dataDir, name)
		if f, err := os.Open(p); err == nil {
			return f, nil
		}
	}
	if extDir := os.Getenv("GORO_DATA_DIR"); extDir != "" {
		p := filepath.Join(extDir, name)
		if f, err := os.Open(p); err == nil {
			return f, nil
		}
	}
	return os.Open(name)
}

func (al *AndroidAssetLoader) SetDataDir(path string) {
	al.dataDir = path
}
