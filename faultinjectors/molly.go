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
	Run         string
	OutputDir   string
	Runs        []*Run
	SuccessRuns []uint
	FailedRuns  []uint
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

	m.SuccessRuns = make([]uint, 0, len(m.Runs))
	m.FailedRuns = make([]uint, 0, 3)

	// Load pre- and post-provenance for each iteration.
	for i := range m.Runs {

		// Note return status of fault injection
		// run in separate structure.
		if m.Runs[i].Status == "success" {
			m.SuccessRuns = append(m.SuccessRuns, m.Runs[i].Iteration)
		} else {
			m.FailedRuns = append(m.FailedRuns, m.Runs[i].Iteration)
		}

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

		// Prefix goals with "pre_".
		for j := range m.Runs[i].PreProv.Goals {
			m.Runs[i].PreProv.Goals[j].ID = fmt.Sprintf("pre_%s", m.Runs[i].PreProv.Goals[j].ID)
		}

		// Prefix rules with "pre_".
		for j := range m.Runs[i].PreProv.Rules {
			m.Runs[i].PreProv.Rules[j].ID = fmt.Sprintf("pre_%s", m.Runs[i].PreProv.Rules[j].ID)
		}

		// Prefix edges with "pre_".
		for j := range m.Runs[i].PreProv.Edges {
			m.Runs[i].PreProv.Edges[j].From = fmt.Sprintf("pre_%s", m.Runs[i].PreProv.Edges[j].From)
			m.Runs[i].PreProv.Edges[j].To = fmt.Sprintf("pre_%s", m.Runs[i].PreProv.Edges[j].To)
		}

		rawPostProvCont, err := ioutil.ReadFile(postProvFile)
		if err != nil {
			return fmt.Errorf("Failed reading post-provenance of file '%v': %v", postProvFile, err)
		}

		err = json.Unmarshal(rawPostProvCont, &m.Runs[i].PostProv)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal JSON post-provenance data: %v\n", err)
		}

		// Prefix goals with "post_".
		for j := range m.Runs[i].PostProv.Goals {
			m.Runs[i].PostProv.Goals[j].ID = fmt.Sprintf("post_%s", m.Runs[i].PostProv.Goals[j].ID)
		}

		// Prefix rules with "post_".
		for j := range m.Runs[i].PostProv.Rules {
			m.Runs[i].PostProv.Rules[j].ID = fmt.Sprintf("post_%s", m.Runs[i].PostProv.Rules[j].ID)
		}

		// Prefix edges with "post_".
		for j := range m.Runs[i].PostProv.Edges {
			m.Runs[i].PostProv.Edges[j].From = fmt.Sprintf("post_%s", m.Runs[i].PostProv.Edges[j].From)
			m.Runs[i].PostProv.Edges[j].To = fmt.Sprintf("post_%s", m.Runs[i].PostProv.Edges[j].To)
		}
	}

	return nil
}

// GetOutput returns all parsed runs from Molly.
func (m *Molly) GetOutput() []*Run {
	return m.Runs
}

// GetFailedRuns returns indexes of failed runs.
func (m *Molly) GetFailedRuns() []uint {
	return m.FailedRuns
}
