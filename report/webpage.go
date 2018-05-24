package report

import (
	"fmt"
	"os"
	"strings"

	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/awalterschulze/gographviz"
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

// GenerateFigure renders a supplied dot graph.
func (r *Report) GenerateFigure(fileName string, dotProv *gographviz.Graph) error {

	dotFilePath := filepath.Join(r.figuresDir, fmt.Sprintf("%s.dot", fileName))
	svgFilePath := filepath.Join(r.figuresDir, fmt.Sprintf("%s.svg", fileName))

	// Write-out file containing DOT string.
	err := ioutil.WriteFile(dotFilePath, []byte(dotProv.String()), 0644)
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

	return nil
}

// GenerateFigures
func (r *Report) GenerateFigures(iters []uint, name string, dotProvs []*gographviz.Graph) error {

	// We require that each element in dotProvs
	// has a corresponding element in names.
	if len(iters) != len(dotProvs) {
		return fmt.Errorf("Unequal number of iteration numbers and DOT graph strings")
	}

	for i := range iters {

		fileName := fmt.Sprintf("run_%d_%s", iters[i], name)

		// Generate and write-out figure.
		err := r.GenerateFigure(fileName, dotProvs[i])
		if err != nil {
			return err
		}
	}

	return nil
}
