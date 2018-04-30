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
	Type  string `json:"-"`
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
	Iteration     uint            `json:"iteration"`
	Status        string          `json:"status"`
	FailureSpec   *FailureSpec    `json:"failureSpec"`
	Model         *Model          `json:"model"`
	Messages      []*Message      `json:"messages"`
	MessagesIndex map[string]bool `json:"-"`
	PreProv       *ProvData       `json:"-"`
	PostProv      *ProvData       `json:"-"`
}

// Molly
type Molly struct {
	Run              string
	OutputDir        string
	Runs             []*Run
	RunsIters        []uint
	SuccessRunsIters []uint
	FailedRunsIters  []uint
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

	m.RunsIters = make([]uint, len(m.Runs))
	m.SuccessRunsIters = make([]uint, 0, len(m.Runs))
	m.FailedRunsIters = make([]uint, 0, 3)

	// Load pre- and post-provenance for each iteration.
	for i := range m.Runs {

		// Note return status of fault injection
		// run in separate structure.
		m.RunsIters[i] = m.Runs[i].Iteration
		if m.Runs[i].Status == "success" {
			m.SuccessRunsIters = append(m.SuccessRunsIters, m.Runs[i].Iteration)
		} else {
			m.FailedRunsIters = append(m.FailedRunsIters, m.Runs[i].Iteration)
		}

		m.Runs[i].MessagesIndex = make(map[string]bool)
		for j := range m.Runs[i].Messages {
			m.Runs[i].MessagesIndex[m.Runs[i].Messages[j].Content] = true
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

		for j := range m.Runs[i].PreProv.Goals {

			// Prefix goals with "pre_".
			m.Runs[i].PreProv.Goals[j].ID = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Goals[j].ID)

			// Set type of goal to either:
			// * async => message passing event (@async)
			// * next => local state chain (@next) (TODO)
			// * single => one-time local event
			_, isMsg := m.Runs[i].MessagesIndex[m.Runs[i].PreProv.Goals[j].Table]
			if isMsg {
				m.Runs[i].PreProv.Goals[j].Type = "async"
			} else {
				m.Runs[i].PreProv.Goals[j].Type = "single"
			}
		}

		for j := range m.Runs[i].PreProv.Rules {

			// Prefix rules with "pre_".
			m.Runs[i].PreProv.Rules[j].ID = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Rules[j].ID)

			// Set type of rule to either:
			// * async => message passing event (@async)
			// * next => local state chain (@next) (TODO)
			// * single => one-time local event
			_, isMsg := m.Runs[i].MessagesIndex[m.Runs[i].PreProv.Rules[j].Table]
			if isMsg {
				m.Runs[i].PreProv.Rules[j].Type = "async"
			} else {
				m.Runs[i].PreProv.Rules[j].Type = "single"
			}
		}

		// Prefix edges with "pre_".
		for j := range m.Runs[i].PreProv.Edges {
			m.Runs[i].PreProv.Edges[j].From = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Edges[j].From)
			m.Runs[i].PreProv.Edges[j].To = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Edges[j].To)
		}

		rawPostProvCont, err := ioutil.ReadFile(postProvFile)
		if err != nil {
			return fmt.Errorf("Failed reading post-provenance of file '%v': %v", postProvFile, err)
		}

		err = json.Unmarshal(rawPostProvCont, &m.Runs[i].PostProv)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal JSON post-provenance data: %v\n", err)
		}

		for j := range m.Runs[i].PostProv.Goals {

			// Prefix goals with "post_".
			m.Runs[i].PostProv.Goals[j].ID = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Goals[j].ID)

			// Set type of goal to either:
			// * async => message passing event (@async)
			// * next => local state chain (@next) (TODO)
			// * single => one-time local event
			_, isMsg := m.Runs[i].MessagesIndex[m.Runs[i].PostProv.Goals[j].Table]
			if isMsg {
				m.Runs[i].PostProv.Goals[j].Type = "async"
			} else {
				m.Runs[i].PostProv.Goals[j].Type = "single"
			}
		}

		for j := range m.Runs[i].PostProv.Rules {

			// Prefix rules with "post_".
			m.Runs[i].PostProv.Rules[j].ID = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Rules[j].ID)

			// Set type of rule to either:
			// * async => message passing event (@async)
			// * next => local state chain (@next) (TODO)
			// * single => one-time local event
			_, isMsg := m.Runs[i].MessagesIndex[m.Runs[i].PostProv.Rules[j].Table]
			if isMsg {
				m.Runs[i].PostProv.Rules[j].Type = "async"
			} else {
				m.Runs[i].PostProv.Rules[j].Type = "single"
			}
		}

		// Prefix edges with "post_".
		for j := range m.Runs[i].PostProv.Edges {
			m.Runs[i].PostProv.Edges[j].From = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Edges[j].From)
			m.Runs[i].PostProv.Edges[j].To = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Edges[j].To)
		}
	}

	return nil
}

// GetOutput returns all parsed runs from Molly.
func (m *Molly) GetOutput() []*Run {
	return m.Runs
}

// GetRunsIters returns the iteration numbers
// of all runs known in this struct.
func (m *Molly) GetRunsIters() []uint {
	return m.RunsIters
}

// GetFailedRunsIters returns indexes of failed runs.
func (m *Molly) GetFailedRunsIters() []uint {
	return m.FailedRunsIters
}
