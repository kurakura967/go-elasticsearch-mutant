package report

import (
	"encoding/json"
	"io"
)

// PrintJSON writes the mutation report as indented JSON to w.
func PrintJSON(w io.Writer, summary Summary, details []MutantDetail) error {
	type mutantJSON struct {
		ID          int    `json:"id"`
		Status      string `json:"status"`
		Operator    string `json:"operator"`
		Description string `json:"description"`
		File        string `json:"file"`
		Line        int    `json:"line"`
	}
	type reportJSON struct {
		Score    float64      `json:"score"`
		Total    int          `json:"total"`
		Killed   int          `json:"killed"`
		Survived int          `json:"survived"`
		Timeouts int          `json:"timeouts"`
		Errors   int          `json:"errors"`
		Mutants  []mutantJSON `json:"mutants"`
	}

	muts := make([]mutantJSON, len(details))
	for i, d := range details {
		muts[i] = mutantJSON{
			ID:          d.ID,
			Status:      d.Status.String(),
			Operator:    d.Operator,
			Description: d.Description,
			File:        d.File,
			Line:        d.Line,
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(reportJSON{
		Score:    summary.Score(),
		Total:    summary.Total,
		Killed:   summary.Killed,
		Survived: summary.Survived,
		Timeouts: summary.Timeouts,
		Errors:   summary.Errors,
		Mutants:  muts,
	})
}
