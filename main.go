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
	GetOutput() []*faultinjectors.Run
}

// GraphDatabase
type GraphDatabase interface {
	InitGraphDB(string) error
	CloseDB() error
	LoadNaiveProv([]*faultinjectors.Run) error
}

// Structs.

// DebugRun
type DebugRun struct {
	workDir         string
	allResultsDir   string
	thisResultsDir  string
	tmpGraphDBDir   string
	tmpGraphLogsDir string
	faultInj        FaultInjector
	graphDB         GraphDatabase
}

func main() {

	// Define which flags are supported.
	faultInjOutFlag := flag.String("faultInjOut", "", "Specify file system path to output directory of fault injector.")
	graphDBConnFlag := flag.String("graphDBConn", "bolt://127.0.0.1:7687", "Supply connection URI to dockerized graph database.")
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
		workDir:         curDir,
		allResultsDir:   filepath.Join(curDir, "results"),
		thisResultsDir:  filepath.Join(curDir, "results", filepath.Base(faultInjOut)),
		tmpGraphDBDir:   filepath.Join(curDir, "tmp", "db"),
		tmpGraphLogsDir: filepath.Join(curDir, "tmp", "logs"),
		faultInj: &faultinjectors.Molly{
			Run:       filepath.Base(faultInjOut),
			OutputDir: faultInjOut,
		},
		graphDB: &graphing.Neo4J{},
	}

	// Ensure the results directory for this debug run exists.
	err = os.MkdirAll(debugRun.thisResultsDir, 0755)
	if err != nil {
		log.Fatalf("Could not ensure resDir exists: %v", err)
	}

	// Empty temporary directory for graph data.
	err = os.RemoveAll(filepath.Join(curDir, "tmp"))
	if err != nil {
		log.Fatalf("Could not remove temporary graph database directory: %v", err)
	}

	// Make sure temporary directory exists for graph data.
	err = os.MkdirAll(debugRun.tmpGraphDBDir, 0755)
	if err != nil {
		log.Fatalf("Could not ensure ./tmp/db exists: %v", err)
	}

	err = os.MkdirAll(debugRun.tmpGraphLogsDir, 0755)
	if err != nil {
		log.Fatalf("Could not ensure ./tmp/logs exists: %v", err)
	}

	// Extract, transform, and load fault injector output.
	err = debugRun.faultInj.LoadOutput()
	if err != nil {
		log.Fatalf("Failed to load output from Molly: %v", err)
	}

	// Connect to graph database docker container.
	err = debugRun.graphDB.InitGraphDB(graphDBConn)
	if err != nil {
		log.Fatalf("Failed to initialize connection to graph database: %v", err)
	}
	defer debugRun.graphDB.CloseDB()

	// Prepare and calculate provenance graphs.
	err = debugRun.graphDB.LoadNaiveProv(debugRun.faultInj.GetOutput())
	if err != nil {
		log.Fatalf("Failed to import provenance (naive) into graph database: %v", err)
	}

	// Analyze (debug) the system.

	// Create and write-out report.
}
