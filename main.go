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
)

// Interfaces.

// FaultInjector
type FaultInjector interface {
	LoadOutput(string) error
}

// GraphDatabase
type GraphDatabase interface {
	LoadNaiveProv() error
}

// Structs.

type CrashFailure struct {
	Node string
	Time uint
}

type MessageLoss struct {
	From string
	To   string
	Time uint
}

// FailureSpec
type FailureSpec struct {
	EOT        uint
	EFF        uint
	MaxCrashes uint
	Nodes      *[]string
	Crashes    *[]CrashFailure
	Omissions  *[]MessageLoss
}

// Model
type Model struct {
	Tables map[string][][]string
}

// Message
type Message struct {
	Content  string `json:"table"`
	SendNode string `json:"from"`
	RecvNode string `json:"to"`
	SendTime uint   `json:"sendTime"`
	RecvTime uint   `json:"receiveTime"`
}

// FaultInjRun
type FaultInjRun struct {
	Iteration   uint         `json:"iteration"`
	Status      string       `json:"status"`
	FailureSpec *FailureSpec `json:"failureSpec"`
	Model       *Model       `json:"model"`
	Messages    []*Message   `json:"messages"`
	PreProv     *ProvData    `json:"-"`
	PostProv    *ProvData    `json:"-"`
}

// DebugRun
type DebugRun struct {
	workDir        string
	allResultsDir  string
	thisResultsDir string
	faultInj       FaultInjector
	graphDB        GraphDatabase
	faultInjRuns   []*FaultInjRun
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

	// Find out how many iterations the fault injection run contains.
	rawRunsCont, err := ioutil.ReadFile(filepath.Join(faultInjOut, "runs.json"))
	if err != nil {
		log.Fatalf("Could not read runs.json file in faultInjOut directory: %v", err)
	}

	err = json.Unmarshal(rawRunsCont, &debugRun.faultInjRuns)
	if err != nil {
		log.Fatalf("Failed to unmarshal JSON content to runs structure: %v\n", err)
	}

	for i := range debugRun.faultInjRuns {
		fmt.Printf("\trun %d: '%v'\n", i, debugRun.faultInjRuns[i])
	}

	// Extract, transform, and load fault injector output.
	err = debugRun.faultInj.LoadOutput(debugRun.thisResultsDir)
	if err != nil {
		log.Fatalf("Failed to load output from Molly: %v", err)
	}

	// Prepare and calculate provenance graphs.

	// Analyze (debug) the system.

	// Create and write-out report.
}
