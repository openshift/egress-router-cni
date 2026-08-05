package testdata

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed all:egressrouter all:adminnetworkpolicy ping-for-pod-template.yaml
var embeddedFS embed.FS

var (
	extractOnce sync.Once
	extractDir  string
	extractErr  error
)

func FixturePath(elem ...string) string {
	extractOnce.Do(func() {
		dir, err := os.MkdirTemp("", "otp-testdata-")
		if err != nil {
			extractErr = fmt.Errorf("failed to create temp dir: %w", err)
			return
		}
		if err := fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			target := filepath.Join(dir, path)
			if d.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			data, err := embeddedFS.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0644)
		}); err != nil {
			extractErr = fmt.Errorf("failed to extract test fixtures: %w", err)
			return
		}
		extractDir = dir
	})
	if extractErr != nil {
		panic(extractErr)
	}
	return filepath.Join(append([]string{extractDir}, elem...)...)
}
