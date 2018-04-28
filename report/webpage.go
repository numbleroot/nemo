package report

import (
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Structs.

// Run
type Run struct {
	Iteration   uint            `json:"iteration"`
	Status      string          `json:"status"`
	Suggestions []string        `json:"suggestions"`
	FailureSpec *fi.FailureSpec `json:"failureSpec"`
}

// Report
type Report struct {
	Runs []*Run `json:"runs"`
}

// Functions.

// GenerateReport
func (r *Report) GenerateReport() error {
	return nil
}
