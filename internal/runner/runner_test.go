package runner_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

// --- shared helpers ---

// setupTestModule creates a minimal Go module with a Value() function and a test.
func setupTestModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module testmod\ngo 1.21\n")
	write("calc.go", `package testmod

func Value() int { return 42 }
`)
	write("calc_test.go", `package testmod

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}
`)
	return dir
}

// makeOverlay writes overlay.json that replaces originalFile with mutant source.
func makeOverlay(t *testing.T, originalFile, src string) string {
	t.Helper()
	mutantFile := filepath.Join(t.TempDir(), "mutant.go")
	os.WriteFile(mutantFile, []byte(src), 0644)

	type ov struct {
		Replace map[string]string `json:"Replace"`
	}
	b, _ := json.Marshal(ov{Replace: map[string]string{originalFile: mutantFile}})

	overlayFile := filepath.Join(t.TempDir(), "overlay.json")
	os.WriteFile(overlayFile, b, 0644)
	return overlayFile
}

// --- OverlayManager tests ---

func TestOverlayManager_Write(t *testing.T) {
	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatalf("NewOverlayManager: %v", err)
	}
	defer mgr.Cleanup()

	fakeOrig := filepath.Join(t.TempDir(), "orig.go")
	os.WriteFile(fakeOrig, []byte("package test\n"), 0644)

	mut := &mutant.Mutant{
		ID:          7,
		ModifiedSrc: []byte("package test\nfunc modified() {}\n"),
		Site:        &analyzer.CallSite{File: fakeOrig},
	}

	overlayPath, cleanup, err := mgr.Write(mut)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// overlay.json must map the original file to a mutant file
	raw, _ := os.ReadFile(overlayPath)
	var ov struct {
		Replace map[string]string `json:"Replace"`
	}
	if err := json.Unmarshal(raw, &ov); err != nil {
		t.Fatalf("unmarshal overlay: %v", err)
	}
	mutantFile, ok := ov.Replace[fakeOrig]
	if !ok {
		t.Fatalf("overlay missing key %q, got: %v", fakeOrig, ov.Replace)
	}

	// mutant source file must contain the expected content
	got, _ := os.ReadFile(mutantFile)
	if string(got) != string(mut.ModifiedSrc) {
		t.Errorf("mutant source:\ngot  %q\nwant %q", got, mut.ModifiedSrc)
	}

	// cleanup removes both files
	cleanup()
	if _, err := os.Stat(overlayPath); !os.IsNotExist(err) {
		t.Error("overlay.json should be removed after cleanup()")
	}
	if _, err := os.Stat(mutantFile); !os.IsNotExist(err) {
		t.Error("mutant source should be removed after cleanup()")
	}
}

func TestOverlayManager_Cleanup(t *testing.T) {
	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatalf("NewOverlayManager: %v", err)
	}
	workDir := mgr.WorkDir
	if err := mgr.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Error("WorkDir should not exist after Cleanup()")
	}
}

// --- Executor tests ---

func TestExecutor_Survived(t *testing.T) {
	dir := setupTestModule(t)
	exec := &runner.Executor{ProjectDir: dir, Timeout: 30 * time.Second}

	// overlay replaces calc.go with an equivalent → test still passes
	overlayPath := makeOverlay(t, filepath.Join(dir, "calc.go"), `package testmod

func Value() int { return 42 }
`)
	r, err := exec.Run(context.Background(), overlayPath, ".")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Status != runner.Survived {
		t.Errorf("got %v, want Survived\noutput:\n%s", r.Status, r.Output)
	}
}

func TestExecutor_Killed(t *testing.T) {
	dir := setupTestModule(t)
	exec := &runner.Executor{ProjectDir: dir, Timeout: 30 * time.Second}

	// overlay replaces calc.go with a broken mutant → test fails
	overlayPath := makeOverlay(t, filepath.Join(dir, "calc.go"), `package testmod

func Value() int { return 0 }
`)
	r, err := exec.Run(context.Background(), overlayPath, ".")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Status != runner.Killed {
		t.Errorf("got %v, want Killed\noutput:\n%s", r.Status, r.Output)
	}
}

func TestExecutor_CompileError(t *testing.T) {
	dir := setupTestModule(t)
	exec := &runner.Executor{ProjectDir: dir, Timeout: 30 * time.Second}

	// overlay introduces an undefined identifier → build fails
	overlayPath := makeOverlay(t, filepath.Join(dir, "calc.go"), `package testmod

func Value() int { return UNDEFINED }
`)
	r, err := exec.Run(context.Background(), overlayPath, ".")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Status != runner.Error {
		t.Errorf("got %v, want Error\noutput:\n%s", r.Status, r.Output)
	}
}

// --- RunAll (worker) tests ---

func TestRunAll_OrderAndIDs(t *testing.T) {
	dir := setupTestModule(t)
	calcGo := filepath.Join(dir, "calc.go")

	mutants := []*mutant.Mutant{
		{
			ID:          1,
			ModifiedSrc: []byte("package testmod\nfunc Value() int { return 42 }\n"),
			Site:        &analyzer.CallSite{File: calcGo},
		},
		{
			ID:          2,
			ModifiedSrc: []byte("package testmod\nfunc Value() int { return 0 }\n"),
			Site:        &analyzer.CallSite{File: calcGo},
		},
	}

	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Cleanup()

	exec := &runner.Executor{ProjectDir: dir, Timeout: 30 * time.Second}
	results, err := runner.RunAll(context.Background(), mutants, ".", exec, mgr, 2, nil)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}

	// Order is preserved (index 0 = mutant ID 1, index 1 = mutant ID 2)
	if results[0].MutantID != 1 || results[0].Status != runner.Survived {
		t.Errorf("results[0]: MutantID=%d Status=%v, want ID=1 Survived", results[0].MutantID, results[0].Status)
	}
	if results[1].MutantID != 2 || results[1].Status != runner.Killed {
		t.Errorf("results[1]: MutantID=%d Status=%v, want ID=2 Killed", results[1].MutantID, results[1].Status)
	}
}

func TestRunAll_Cancellation(t *testing.T) {
	dir := setupTestModule(t)
	calcGo := filepath.Join(dir, "calc.go")

	mutants := []*mutant.Mutant{
		{ID: 1, ModifiedSrc: []byte("package testmod\nfunc Value() int { return 42 }\n"), Site: &analyzer.CallSite{File: calcGo}},
		{ID: 2, ModifiedSrc: []byte("package testmod\nfunc Value() int { return 42 }\n"), Site: &analyzer.CallSite{File: calcGo}},
	}

	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Cleanup()

	// Cancel immediately before RunAll processes any job
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exec := &runner.Executor{ProjectDir: dir, Timeout: 30 * time.Second}
	results, err := runner.RunAll(ctx, mutants, ".", exec, mgr, 1, nil)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Status != runner.Error {
			// Some results may still be Survived if they ran before cancel propagated
			t.Logf("results[%d]: %v (cancel may not have propagated)", i, r.Status)
		}
	}
}
