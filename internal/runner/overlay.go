package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
)

// OverlayManager writes mutant sources and overlay.json files to a working directory.
type OverlayManager struct {
	WorkDir string
}

// NewOverlayManager creates an OverlayManager with a fresh temporary working directory.
func NewOverlayManager() (*OverlayManager, error) {
	dir, err := os.MkdirTemp("", "esmutant_*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	return &OverlayManager{WorkDir: dir}, nil
}

// Cleanup removes the working directory and all its contents.
func (m *OverlayManager) Cleanup() error {
	return os.RemoveAll(m.WorkDir)
}

type overlayJSON struct {
	Replace map[string]string `json:"Replace"`
}

// Write writes the mutant source to a temp file and generates an overlay.json.
// The returned cleanup removes both files (but not the WorkDir itself).
func (m *OverlayManager) Write(mut *mutant.Mutant) (overlayPath string, cleanup func(), err error) {
	srcFile := filepath.Join(m.WorkDir, fmt.Sprintf("mutant_%d.go", mut.ID))
	if err := os.WriteFile(srcFile, mut.ModifiedSrc, 0644); err != nil {
		return "", nil, fmt.Errorf("write mutant source: %w", err)
	}

	b, err := json.Marshal(overlayJSON{
		Replace: map[string]string{mut.Site.File: srcFile},
	})
	if err != nil {
		os.Remove(srcFile)
		return "", nil, fmt.Errorf("marshal overlay: %w", err)
	}

	overlayFile := filepath.Join(m.WorkDir, fmt.Sprintf("overlay_%d.json", mut.ID))
	if err := os.WriteFile(overlayFile, b, 0644); err != nil {
		os.Remove(srcFile)
		return "", nil, fmt.Errorf("write overlay json: %w", err)
	}

	cleanup = func() {
		os.Remove(srcFile)
		os.Remove(overlayFile)
	}

	return overlayFile, cleanup, nil
}
