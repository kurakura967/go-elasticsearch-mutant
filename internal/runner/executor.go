package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Executor runs go test with an overlay for a single mutant.
type Executor struct {
	ProjectDir string
	Timeout    time.Duration // per-run timeout; 0 means no additional timeout
}

// Run executes: go test -json -overlay overlayPath -count=1 pattern
// All outcomes (killed, survived, timeout, compile error) are captured in Result.
func (e *Executor) Run(ctx context.Context, overlayPath, pattern string) (Result, error) {
	if e.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-overlay", overlayPath, "-count=1", pattern)
	cmd.Dir = e.ProjectDir
	out, err := cmd.CombinedOutput()
	raw := string(out)

	testResults, humanOutput := parseTestJSON(raw)

	if ctx.Err() == context.DeadlineExceeded {
		return Result{Status: Timeout, Output: humanOutput, TestResults: testResults}, nil
	}

	if err == nil {
		return Result{Status: Survived, Output: humanOutput, TestResults: testResults}, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if strings.Contains(raw, "[build failed]") {
			return Result{Status: Error, Output: humanOutput, TestResults: testResults}, nil
		}
		return Result{Status: Killed, Output: humanOutput, TestResults: testResults}, nil
	}

	return Result{Status: Error, Output: fmt.Sprintf("exec: %v\n%s", err, humanOutput)}, nil
}
