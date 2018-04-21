package main

import (
	"flag"
	"log"
	"os"

	"path/filepath"

	"github.com/numbleroot/nemo/faultinjectors"
	"github.com/numbleroot/nemo/graphing"
)

// Interfaces.

// FaultInjector
type FaultInjector interface {
	LoadOutput() error
}

// GraphDatabase
type GraphDatabase interface {
	InitGraphDB(string) error
	LoadNaiveProv() error
}

// Structs.

// DebugRun
type DebugRun struct {
	workDir        string
	allResultsDir  string
	thisResultsDir string
	faultInj       FaultInjector
	graphDB        GraphDatabase
}

func main() {

	// Define which flags are supported.
	faultInjOutFlag := flag.String("faultInjOut", "", "Specify file system path to output directory of fault injector.")
	graphDBConnFlag := flag.String("graphDBConn", "bolt://127.0.0.1:7687", "Supply URI to use for connecting to graph database.")
	flag.Parse()

	// Extract and check for existence of required ones.
	faultInjOut := *faultInjOutFlag
	if faultInjOut == "" {
		log.Fatal("Please provide a fault injection output directory to analyze.")
	}

	graphDBConn := *graphDBConnFlag

	// Determine current working directory.
	curDir, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("Failed obtaining absolute current directory: %v", err)
	}

	// Start building structs.
	debugRun := &DebugRun{
		faultInj: &faultinjectors.Molly{
			Run:       filepath.Base(faultInjOut),
			OutputDir: faultInjOut,
		},
		graphDB:        &graphing.Neo4J{},
		workDir:        curDir,
		allResultsDir:  filepath.Join(curDir, "results"),
		thisResultsDir: filepath.Join(curDir, "results", filepath.Base(faultInjOut)),
	}

	// Ensure the results directory for this debug run exists.
	err = os.MkdirAll(debugRun.thisResultsDir, 0755)
	if err != nil {
		log.Fatalf("Could not ensure resDir existence: %v", err)
	}

	// Extract, transform, and load fault injector output.
	err = debugRun.faultInj.LoadOutput()
	if err != nil {
		log.Fatalf("Failed to load output from Molly: %v", err)
	}

	// Connect to graph database.
	err = debugRun.graphDB.InitGraphDB(graphDBConn)
	if err != nil {
		log.Fatalf("Failed to initialize connection to graph database: %v", err)
	}

	// Prepare and calculate provenance graphs.
	err = debugRun.graphDB.LoadNaiveProv()
	if err != nil {
		log.Fatalf("Failed to import provenance (naive) into graph database: %v", err)
	}

	// Analyze (debug) the system.

	// Create and write-out report.
}
