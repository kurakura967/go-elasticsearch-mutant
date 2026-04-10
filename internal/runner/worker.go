package runner

import (
	"context"
	"sync"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
)

// RunAll runs all mutants in parallel using numWorkers goroutines.
// Results are returned in the same order as the input mutants slice.
// onResult is called after each mutant completes (may be nil).
// Cancelling ctx stops pending work; already-running tests complete on their own.
func RunAll(ctx context.Context, mutants []*mutant.Mutant, pattern string, exec *Executor, mgr *OverlayManager, numWorkers int, onResult func(Result)) ([]Result, error) {
	if numWorkers <= 0 {
		numWorkers = 4
	}

	type job struct {
		idx int
		mut *mutant.Mutant
	}

	// Pre-buffer all jobs so workers can drain without a producer goroutine.
	jobs := make(chan job, len(mutants))
	for i, m := range mutants {
		jobs <- job{i, m}
	}
	close(jobs)

	results := make([]Result, len(mutants))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var r Result
				if ctx.Err() != nil {
					r = Result{MutantID: j.mut.ID, Status: Error, Output: ctx.Err().Error()}
				} else {
					r = runOne(ctx, j.mut, pattern, exec, mgr)
				}
				mu.Lock()
				results[j.idx] = r
				if onResult != nil {
					onResult(r)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results, nil
}

func runOne(ctx context.Context, mut *mutant.Mutant, pattern string, exec *Executor, mgr *OverlayManager) Result {
	if mut.SkipReason != "" {
		return Result{MutantID: mut.ID, Status: Skipped, Output: mut.SkipReason}
	}

	overlayPath, cleanup, err := mgr.Write(mut)
	if err != nil {
		return Result{MutantID: mut.ID, Status: Error, Output: err.Error()}
	}
	defer cleanup()

	r, err := exec.Run(ctx, overlayPath, pattern)
	if err != nil {
		return Result{MutantID: mut.ID, Status: Error, Output: err.Error()}
	}
	r.MutantID = mut.ID
	return r
}
