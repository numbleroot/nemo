package report

import (
	"os"

	"path/filepath"

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
func (r *Report) GenerateReport(wrkDir string, allResDir string, thisResDir string) error {

	// Copy webpage template to result directory.
	err := copyDir(filepath.Join(wrkDir, "report", "assets"), allResDir)
	if err != nil {
		return err
	}

	// Rename to final results directory name.
	err = os.Rename(filepath.Join(allResDir, "assets"), thisResDir)
	if err != nil {
		return err
	}

	return nil
}
