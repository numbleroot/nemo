package main

import (
	"flag"
	"log"
	"os"

	"path/filepath"
)

func main() {

	// Define which flags are supported.
	faultInjOutFlag := flag.String("faultInjOut", "", "Specify file system path to output directory of fault injector.")
	flag.Parse()

	// Extract and check for existence of required ones.
	faultInjOut := *faultInjOutFlag
	if faultInjOut == "" {
		log.Fatal("Please provide a fault injection output directory to analyze.")
	}

	// Determine current working directory.
	curDir, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("Failed obtaining absolute current directory: %v", err)
	}

	// Define currently non-configurable variables.
	faultInjRun := filepath.Base(faultInjOut)
	allResDir := filepath.Join(curDir, "results")
	resDir := filepath.Join(allResDir, faultInjRun)

	// Ensure the results directory for this debug run exists.
	err = os.MkdirAll(resDir, 0755)
	if err != nil {
		log.Fatalf("Could not ensure resDir existence: %v", err)
	}

	// Extract, transform, and load fault injector output.

	// Prepare and calculate provenance graphs.

	// Analyze (debug) the system.

	// Create and write-out report.
}
