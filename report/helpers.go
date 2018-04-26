package report

import (
	"fmt"
	"strings"

	"os/exec"
)

// CopyFaultInjReport
func (r *Report) CopyFaultInjReport(srcDir string, resDir string) error {

	fmt.Printf("Copying fault injector's results...")
	cmd := exec.Command("cp", "-r", srcDir, resDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from copy command for results: %s", out)
	}
	fmt.Printf(" done\n\n")

	return nil
}
