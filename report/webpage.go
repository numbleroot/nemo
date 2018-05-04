package report

import (
	"fmt"
	"os"
	"strings"

	"io/ioutil"
	"os/exec"
	"path/filepath"
)

// Structs.

// Report
type Report struct {
	resDir     string
	figuresDir string
}

// Functions.

// Prepare
func (r *Report) Prepare(wrkDir string, allResDir string, thisResDir string) error {

	r.resDir = thisResDir
	r.figuresDir = filepath.Join(thisResDir, "figures")

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

	// Create directory to hold diagrams.
	err = os.MkdirAll(r.figuresDir, 0755)
	if err != nil {
		return err
	}

	return nil
}

// GenerateGraphs
func (r *Report) GenerateGraphs(iters []uint, name string, dotProv []string) error {

	// We require that each element in dotProv
	// has a corresponding element in names.
	if len(iters) != len(dotProv) {
		return fmt.Errorf("Unequal number of iteration numbers and DOT graph strings")
	}

	for i := range dotProv {

		dotFilePath := filepath.Join(r.figuresDir, fmt.Sprintf("run_%d_%s.dot", iters[i], name))
		svgFilePath := filepath.Join(r.figuresDir, fmt.Sprintf("run_%d_%s.svg", iters[i], name))

		// Write-out file containing DOT string.
		err := ioutil.WriteFile(dotFilePath, []byte(dotProv[i]), 0644)
		if err != nil {
			return err
		}

		// Run SVG generator on DOT file.
		cmd := exec.Command("dot", "-Tsvg", "-o", svgFilePath, dotFilePath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		if strings.TrimSpace(string(out)) != "" {
			return fmt.Errorf("Wrong return value from SVG generation command: %s", out)
		}
	}

	return nil
}
