package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/numbleroot/nemo/faultinjectors"
	"github.com/numbleroot/nemo/graphing"
	"github.com/numbleroot/nemo/report"
)

// Interfaces.

// FaultInjector
type FaultInjector interface {
	LoadOutput() error
	GetOutput() []*faultinjectors.Run
	GetRunsIters() []uint
	GetFailedRunsIters() []uint
}

// GraphDatabase
type GraphDatabase interface {
	InitGraphDB(string) error
	CloseDB() error
	LoadNaiveProv([]*faultinjectors.Run) error
	PullPrePostProv([]*faultinjectors.Run) ([]string, []string, error)
	CreateNaiveDiffProv(bool, []uint) ([]string, error)
}

// Reporter
type Reporter interface {
	Prepare(string, string, string) error
	GenerateGraphs([]uint, string, []string) error
}

// Structs.

// DebugRun
type DebugRun struct {
	workDir        string
	allResultsDir  string
	thisResultsDir string
	faultInj       FaultInjector
	graphDB        GraphDatabase
	reporter       Reporter
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
		workDir:        curDir,
		allResultsDir:  filepath.Join(curDir, "results"),
		thisResultsDir: filepath.Join(curDir, "results", filepath.Base(faultInjOut)),
		faultInj: &faultinjectors.Molly{
			Run:       filepath.Base(faultInjOut),
			OutputDir: faultInjOut,
		},
		graphDB:  &graphing.Neo4J{},
		reporter: &report.Report{},
	}

	// Ensure the results directory for this debug run exists.
	err = os.MkdirAll(debugRun.allResultsDir, 0755)
	if err != nil {
		log.Fatalf("Could not ensure resDir exists: %v", err)
	}

	// Extract, transform, and load fault injector output.
	err = debugRun.faultInj.LoadOutput()
	if err != nil {
		log.Fatalf("Failed to load output from Molly: %v", err)
	}

	// Graph queries.

	// Connect to graph database docker container.
	err = debugRun.graphDB.InitGraphDB(graphDBConn)
	if err != nil {
		log.Fatalf("Failed to initialize connection to graph database: %v", err)
	}
	defer debugRun.graphDB.CloseDB()

	// Load initial (naive) version of provenance
	// graphs for pre- and postcondition.
	err = debugRun.graphDB.LoadNaiveProv(debugRun.faultInj.GetOutput())
	if err != nil {
		log.Fatalf("Failed to import provenance (naive) into graph database: %v", err)
	}

	// Clean-up loaded provenance data and
	// reimport in reduced versions.
	// TODO: Implement this.
	// err = debugRun.graphDB.PreprocessProv()
	// if err != nil {
	// 	log.Fatalf("Could not clean-up initial provenance data: %v", err)
	// }

	// Extract prototypes of successful and
	// failed runs (skeletons) and import.
	// TODO: Implement this.
	// err = debugRun.graphDB.ExtractPrototypes()
	// if err != nil {
	// 	log.Fatalf("Failed to create prototypical successful and failed executions: %v", err)
	// }

	// Pull pre- and postcondition provenance
	// and create DOT diagram strings.
	preProvDots, postProvDots, err := debugRun.graphDB.PullPrePostProv(debugRun.faultInj.GetOutput())
	if err != nil {
		log.Fatalf("Failed to pull and generate pre- and postcondition provenance DOT: %v", err)
	}

	// Create differential provenance graphs for
	// postcondition provenance.
	naiveDiffProvDots, err := debugRun.graphDB.CreateNaiveDiffProv(false, debugRun.faultInj.GetFailedRunsIters())
	if err != nil {
		log.Fatalf("Could not create the naive differential provenance (bad - good): %v", err)
	}

	// Debugging.

	// Determine correction suggestions (pre ~> diffprov).
	// TODO: Implement this.

	// Determine extension suggestions (diffprov).
	// TODO: Implement this.

	// Reporting.

	// Prepare report webpage containing all insights and suggestions.
	err = debugRun.reporter.Prepare(debugRun.workDir, debugRun.allResultsDir, debugRun.thisResultsDir)
	if err != nil {
		log.Fatalf("Failed to prepare debugging report: %v", err)
	}

	// Marshal collected debugging information to JSON.
	debuggingJSON, err := json.Marshal(debugRun.faultInj.GetOutput())
	if err != nil {
		log.Fatalf("Failed to marshal debugging information to JSON: %v", err)
	}

	// Write debugging JSON to file 'debugging.json'.
	err = ioutil.WriteFile(filepath.Join(debugRun.thisResultsDir, "debugging.json"), debuggingJSON, 0644)
	if err != nil {
		log.Fatalf("Error writing out debugging.json: %v", err)
	}

	// Generate and write-out precondition provenance graphs.
	err = debugRun.reporter.GenerateGraphs(debugRun.faultInj.GetRunsIters(), "pre_prov", preProvDots)
	if err != nil {
		log.Fatalf("Could not generate precondition provenance graph for report: %v", err)
	}

	// Generate and write-out postcondition provenance graphs.
	err = debugRun.reporter.GenerateGraphs(debugRun.faultInj.GetRunsIters(), "post_prov", postProvDots)
	if err != nil {
		log.Fatalf("Could not generate postcondition provenance graph for report: %v", err)
	}

	// Generate and write-out naive differential provenance graphs.
	err = debugRun.reporter.GenerateGraphs(debugRun.faultInj.GetFailedRunsIters(), "diff_post_prov", naiveDiffProvDots)
	if err != nil {
		log.Fatalf("Could not generate naive differential provenance graph for report: %v", err)
	}

	fmt.Printf("All done! Find the debug report here: %s\n\n", filepath.Join(debugRun.thisResultsDir, "index.html"))
}
