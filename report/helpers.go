package report

import (
	"fmt"
	"strings"

	"os/exec"
)

// Functions.

// copyDir
func copyDir(srcDir string, resDir string) error {

	fmt.Printf("Copying %s to %s...", srcDir, resDir)
	cmd := exec.Command("cp", "-r", srcDir, resDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from copy command for directory: %s", out)
	}
	fmt.Printf(" done\n\n")

	return nil
}
