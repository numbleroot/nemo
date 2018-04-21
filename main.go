package main

import (
	"flag"
	"log"
	"os"

	"path/filepath"

	"github.com/joho/godotenv"
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
	flag.Parse()

	// Extract and check for existence of required ones.
	faultInjOut := *faultInjOutFlag
	if faultInjOut == "" {
		log.Fatal("Please provide a fault injection output directory to analyze.")
	}

	// Load content of environment file as variables.
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Failed loading .env file: %v", err)
	}

	// Retrieve graph database connection URI from environment.
	graphDBConn := os.Getenv("GRAPH_DB_URI")
	if graphDBConn == "" {
		log.Fatal("Please set GRAPH_DB_URI in local .env file.")
	}

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
