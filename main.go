package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/awalterschulze/gographviz"
	fi "github.com/numbleroot/nemo/faultinjectors"
	gr "github.com/numbleroot/nemo/graphing"
	re "github.com/numbleroot/nemo/report"
)

// Interfaces.

// FaultInjector
type FaultInjector interface {
	LoadOutput() error
	GetFailureSpec() *fi.FailureSpec
	GetMsgsFailedRuns() [][]*fi.Message
	GetOutput() []*fi.Run
	GetRunsIters() []uint
	GetSuccessRunsIters() []uint
	GetFailedRunsIters() []uint
}

// GraphDatabase
type GraphDatabase interface {
	InitGraphDB(string, []*fi.Run) error
	CloseDB() error
	LoadRawProvenance() error
	SimplifyProv([]uint) error
	CreateHazardAnalysis(string) ([]*gographviz.Graph, error)
	CreatePrototypes([]uint, []uint) ([]string, [][]string, []string, [][]string, error)
	PullPrePostProv() ([]*gographviz.Graph, []*gographviz.Graph, []*gographviz.Graph, []*gographviz.Graph, error)
	CreateNaiveDiffProv(bool, []uint, *gographviz.Graph) ([]*gographviz.Graph, []*gographviz.Graph, [][]*fi.Missing, error)
	GenerateCorrections() ([]string, error)
	GenerateExtensions() ([]string, error)
}

// Reporter
type Reporter interface {
	Prepare(string, string, string) error
	GenerateFigure(string, *gographviz.Graph) error
	GenerateFigures([]uint, string, []*gographviz.Graph) error
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
		faultInj: &fi.Molly{
			Run:       filepath.Base(faultInjOut),
			OutputDir: faultInjOut,
		},
		graphDB:  &gr.Neo4J{},
		reporter: &re.Report{},
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

	// Determine the IDs of all and all failed executions.
	iters := debugRun.faultInj.GetRunsIters()
	failedIters := debugRun.faultInj.GetFailedRunsIters()

	// Connect to graph database docker container.
	err = debugRun.graphDB.InitGraphDB(graphDBConn, debugRun.faultInj.GetOutput())
	if err != nil {
		log.Fatalf("Failed to initialize connection to graph database: %v", err)
	}
	defer debugRun.graphDB.CloseDB()

	// Load initial (naive) version of provenance
	// graphs for pre- and postcondition.
	err = debugRun.graphDB.LoadRawProvenance()
	if err != nil {
		log.Fatalf("Failed to import provenance (naive) into graph database: %v", err)
	}

	// Clean-up loaded provenance data and
	// reimport in reduced versions.
	err = debugRun.graphDB.SimplifyProv(iters)
	if err != nil {
		log.Fatalf("Could not clean-up initial provenance data: %v", err)
	}

	// Create hazard analysis DOT figure.
	hazardDots, err := debugRun.graphDB.CreateHazardAnalysis(faultInjOut)
	if err != nil {
		log.Fatalf("Failed to perform hazard analysis of simulation: %v", err)
	}

	// Extract prototypes of successful and
	// failed runs (skeletons) and import.
	interProto, interProtoMiss, unionProto, unionProtoMiss, err := debugRun.graphDB.CreatePrototypes(debugRun.faultInj.GetSuccessRunsIters(), debugRun.faultInj.GetFailedRunsIters())
	if err != nil {
		log.Fatalf("Failed to create prototypes of successful executions: %v", err)
	}

	// Pull pre- and postcondition provenance
	// and create DOT diagram strings.
	preProvDots, postProvDots, preCleanProvDots, postCleanProvDots, err := debugRun.graphDB.PullPrePostProv()
	if err != nil {
		log.Fatalf("Failed to pull and generate pre- and postcondition provenance DOT: %v", err)
	}

	// Create differential provenance graphs for
	// postcondition provenance.
	naiveDiffDots, naiveFailedDots, missingEvents, err := debugRun.graphDB.CreateNaiveDiffProv(false, debugRun.faultInj.GetFailedRunsIters(), postProvDots[0])
	if err != nil {
		log.Fatalf("Could not create the naive differential provenance (bad - good): %v", err)
	}

	var corrections []string
	if len(failedIters) > 0 {

		// Generate correction suggestions for moving towards correctness.
		corrections, err = debugRun.graphDB.GenerateCorrections()
		if err != nil {
			log.Fatalf("Error while generating corrections: %v", err)
		}
	}

	// Attempt to create extension proposals in case
	// the precondition depends on network events.
	extensions, err := debugRun.graphDB.GenerateExtensions()
	if err != nil {
		log.Fatalf("Error while generating extensions: %v", err)
	}

	// Reporting.

	// Retrieve current state of run output.
	// Enrich with missing events.

	runs := debugRun.faultInj.GetOutput()
	for i := range iters {

		// Progressively formulate one top-level recommendation
		// for programmers to focus on first.
		if len(corrections) > 0 {

			// We observed an invariant violation. Suggest corrections first.
			runs[iters[i]].Recommendation = append(runs[iters[i]].Recommendation, "A fault occurred. Let's try making the protocol correct first.")
			runs[iters[i]].Recommendation = append(runs[iters[i]].Recommendation, corrections...)

		} else if len(extensions) > 0 {

			// In case there exist runs in this execution where the
			// precondition was not achieved (not per se a problem!)
			// and communication had to be performed for the successful
			// run to establish the precondition, it might be a good
			// idea for the system designers to make sure these rules
			// are maximum fault-tolerant.
			runs[iters[i]].Recommendation = append(runs[iters[i]].Recommendation, "All good! No invariant violated. It might make sense to verify the fault tolerance of the following rules, though:")

		} else {

			// No invariant violation happened, no more fault tolerance to add.
			runs[iters[i]].Recommendation = append(runs[iters[i]].Recommendation, "All good! No faults, no missing fault tolerance. Well done!")

		}

		runs[iters[i]].InterProto = interProto
		runs[iters[i]].UnionProto = unionProto
	}

	j := 0
	for i := range failedIters {
		runs[failedIters[i]].Corrections = corrections
		runs[failedIters[i]].MissingEvents = missingEvents[j]
		runs[failedIters[i]].InterProtoMissing = interProtoMiss[j]
		runs[failedIters[i]].UnionProtoMissing = unionProtoMiss[j]
		j++
	}

	// Marshal collected debugging information to JSON.
	debuggingJSON, err := json.Marshal(runs)
	if err != nil {
		log.Fatalf("Failed to marshal debugging information to JSON: %v", err)
	}

	// Prepare report webpage containing all insights and suggestions.
	err = debugRun.reporter.Prepare(debugRun.workDir, debugRun.allResultsDir, debugRun.thisResultsDir)
	if err != nil {
		log.Fatalf("Failed to prepare debugging report: %v", err)
	}

	// Write debugging JSON to file 'debugging.json'.
	err = ioutil.WriteFile(filepath.Join(debugRun.thisResultsDir, "debugging.json"), debuggingJSON, 0644)
	if err != nil {
		log.Fatalf("Error writing out debugging.json: %v", err)
	}

	// Generate and write-out hazard analysis figures.
	err = debugRun.reporter.GenerateFigures(iters, "spacetime", hazardDots)
	if err != nil {
		log.Fatalf("Could not generate hazard analysis figures for report: %v", err)
	}

	// Generate and write-out precondition provenance figures.
	err = debugRun.reporter.GenerateFigures(iters, "pre_prov", preProvDots)
	if err != nil {
		log.Fatalf("Could not generate precondition provenance figures for report: %v", err)
	}

	// Generate and write-out postcondition provenance figures.
	err = debugRun.reporter.GenerateFigures(iters, "post_prov", postProvDots)
	if err != nil {
		log.Fatalf("Could not generate postcondition provenance figures for report: %v", err)
	}

	// Generate and write-out cleaned-up precondition provenance figures.
	err = debugRun.reporter.GenerateFigures(iters, "pre_prov_clean", preCleanProvDots)
	if err != nil {
		log.Fatalf("Could not generate cleaned-up precondition provenance figures for report: %v", err)
	}

	// Generate and write-out cleaned-up postcondition provenance figures.
	err = debugRun.reporter.GenerateFigures(iters, "post_prov_clean", postCleanProvDots)
	if err != nil {
		log.Fatalf("Could not generate cleaned-up postcondition provenance figures for report: %v", err)
	}

	// Generate and write-out naive differential provenance (diff) figures.
	err = debugRun.reporter.GenerateFigures(failedIters, "diff_post_prov-diff", naiveDiffDots)
	if err != nil {
		log.Fatalf("Could not generate naive differential provenance (diff) figures for report: %v", err)
	}

	// Generate and write-out naive differential provenance (failed) figures.
	err = debugRun.reporter.GenerateFigures(failedIters, "diff_post_prov-failed", naiveFailedDots)
	if err != nil {
		log.Fatalf("Could not generate naive differential provenance (failed) figures for report: %v", err)
	}

	fmt.Printf("All done! Find the debug report here: %s\n\n", filepath.Join(debugRun.thisResultsDir, "index.html"))
}
