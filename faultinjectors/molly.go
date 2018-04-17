package faultinjectors

import (
	"fmt"

	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

// Structs.

// CrashFailure
type CrashFailure struct {
	Node string `json:"node"`
	Time uint   `json:"time"`
}

// MessageLoss
type MessageLoss struct {
	From string `json:"from"`
	To   string `json:"to"`
	Time uint   `json:"time"`
}

// FailureSpec
type FailureSpec struct {
	EOT        uint            `json:"eot"`
	EFF        uint            `json:"eff"`
	MaxCrashes uint            `json:"maxCrashes"`
	Nodes      *[]string       `json:"nodes"`
	Crashes    *[]CrashFailure `json:"crashes"`
	Omissions  *[]MessageLoss  `json:"omissions"`
}

// Model
type Model struct {
	Tables map[string][][]string `json:"tables"`
}

// Message
type Message struct {
	Content  string `json:"table"`
	SendNode string `json:"from"`
	RecvNode string `json:"to"`
	SendTime uint   `json:"sendTime"`
	RecvTime uint   `json:"receiveTime"`
}

// Node
type Node struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Table string `json:"table"`
}

// Edge
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ProvData
type ProvData struct {
	Goals []Node `json:"goals"`
	Rules []Node `json:"rules"`
	Edges []Edge `json:"edges"`
}

// Run
type Run struct {
	Iteration   uint         `json:"iteration"`
	Status      string       `json:"status"`
	FailureSpec *FailureSpec `json:"failureSpec"`
	Model       *Model       `json:"model"`
	Messages    []*Message   `json:"messages"`
	PreProv     *ProvData    `json:"-"`
	PostProv    *ProvData    `json:"-"`
}

// Molly
type Molly struct {
	Run       string
	OutputDir string
	Runs      []*Run
}

// Functions.

// LoadOutput
func (m *Molly) LoadOutput() error {

	// Find out how many iterations the fault injection run contains.
	rawRunsCont, err := ioutil.ReadFile(filepath.Join(m.OutputDir, "runs.json"))
	if err != nil {
		return fmt.Errorf("Could not read runs.json file in faultInjOut directory: %v", err)
	}

	// Read runs.json file into structure defined above.
	err = json.Unmarshal(rawRunsCont, &m.Runs)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal JSON content to runs structure: %v\n", err)
	}

	// Load pre- and post-provenance for each iteration.
	for i := range m.Runs {

		preProvFile := filepath.Join(m.OutputDir, fmt.Sprintf("run_%d_pre_provenance.json", i))
		postProvFile := filepath.Join(m.OutputDir, fmt.Sprintf("run_%d_post_provenance.json", i))

		rawPreProvCont, err := ioutil.ReadFile(preProvFile)
		if err != nil {
			return fmt.Errorf("Failed reading pre-provenance of file '%v': %v", preProvFile, err)
		}

		err = json.Unmarshal(rawPreProvCont, &m.Runs[i].PreProv)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal JSON pre-provenance data: %v\n", err)
		}

		rawPostProvCont, err := ioutil.ReadFile(postProvFile)
		if err != nil {
			return fmt.Errorf("Failed reading post-provenance of file '%v': %v", postProvFile, err)
		}

		err = json.Unmarshal(rawPostProvCont, &m.Runs[i].PostProv)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal JSON post-provenance data: %v\n", err)
		}

		fmt.Printf("FILE '%v' => PRE '%v'\n\n", preProvFile, m.Runs[i].PreProv)
		fmt.Printf("FILE '%v' => POST '%v'\n\n", postProvFile, m.Runs[i].PostProv)
	}

	return nil
}
