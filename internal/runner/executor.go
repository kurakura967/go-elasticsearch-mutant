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

// Run executes: go test -overlay overlayPath -count=1 pattern
// All outcomes (killed, survived, timeout, compile error) are captured in Result.
func (e *Executor) Run(ctx context.Context, overlayPath, pattern string) (Result, error) {
	if e.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlayPath, "-count=1", pattern)
	cmd.Dir = e.ProjectDir
	out, err := cmd.CombinedOutput()
	output := string(out)

	if ctx.Err() == context.DeadlineExceeded {
		return Result{Status: Timeout, Output: output}, nil
	}

	if err == nil {
		return Result{Status: Survived, Output: output}, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Build failures (unused variables, type errors, etc.) are counted as
		// Killed: the mutation made the code uncompilable, which is a form of
		// detection. Output is prefixed so reports can distinguish the cause.
		if strings.Contains(output, "[build failed]") {
			return Result{Status: Error, Output: output}, nil
		}
		return Result{Status: Killed, Output: output}, nil
	}

	return Result{Status: Error, Output: fmt.Sprintf("exec: %v\n%s", err, output)}, nil
}
