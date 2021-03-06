package faultinjectors

import (
	"fmt"
	"regexp"

	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

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

	// Load antecedent and consequent provenance for each iteration.
	for i := range m.Runs {

		// Create lookup map for when the
		// antecedent holds in this run.
		m.Runs[i].TimePreHolds = make(map[string]bool)
		for _, table := range m.Runs[i].Model.Tables["pre"] {
			m.Runs[i].TimePreHolds[table[(len(table)-1)]] = true
		}

		// Create lookup map for when the
		// consequent holds in this run.
		m.Runs[i].TimePostHolds = make(map[string]bool)
		for _, table := range m.Runs[i].Model.Tables["post"] {
			m.Runs[i].TimePostHolds[table[(len(table)-1)]] = true
		}

		// Note return status of fault injection
		// run in separate structure.
		m.RunsIters[i] = m.Runs[i].Iteration
		if m.Runs[i].Status == "success" {
			m.SuccessRunsIters = append(m.SuccessRunsIters, m.Runs[i].Iteration)
		} else {
			m.FailedRunsIters = append(m.FailedRunsIters, m.Runs[i].Iteration)
		}

		preProvFile := filepath.Join(m.OutputDir, fmt.Sprintf("run_%d_pre_provenance.json", i))
		postProvFile := filepath.Join(m.OutputDir, fmt.Sprintf("run_%d_post_provenance.json", i))

		rawPreProvCont, err := ioutil.ReadFile(preProvFile)
		if err != nil {
			return fmt.Errorf("Failed reading antecedent provenance of file '%v': %v", preProvFile, err)
		}

		err = json.Unmarshal(rawPreProvCont, &m.Runs[i].PreProv)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal JSON antecedent provenance data: %v\n", err)
		}

		for j := range m.Runs[i].PreProv.Goals {

			if m.Runs[i].PreProv.Goals[j].Table == "clock" {

				clkTimeWildRegex := regexp.MustCompile(`, ([\d]+), __WILDCARD__\)`)
				clkTimeWildMatches := clkTimeWildRegex.FindStringSubmatch(m.Runs[i].PreProv.Goals[j].Label)

				clkTimeTwoRegex := regexp.MustCompile(`, ([\d]+), ([\d]+)\)`)
				clkTimeTwoMatches := clkTimeTwoRegex.FindStringSubmatch(m.Runs[i].PreProv.Goals[j].Label)

				if len(clkTimeWildMatches) > 0 {
					m.Runs[i].PreProv.Goals[j].Time = clkTimeWildMatches[1]
				}

				if len(clkTimeTwoMatches) > 0 {
					m.Runs[i].PreProv.Goals[j].Time = clkTimeTwoMatches[1]
				}
			}

			// Prefix goals with "pre_".
			m.Runs[i].PreProv.Goals[j].ID = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Goals[j].ID)

			// Tentative mark as antecedent not yet achieved
			// until we can do graph operations on this provenance.
			m.Runs[i].PreProv.Goals[j].CondHolds = false
		}

		// Prefix rules with "pre_".
		for j := range m.Runs[i].PreProv.Rules {
			m.Runs[i].PreProv.Rules[j].ID = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Rules[j].ID)
		}

		// Prefix edges with "pre_".
		for j := range m.Runs[i].PreProv.Edges {
			m.Runs[i].PreProv.Edges[j].From = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Edges[j].From)
			m.Runs[i].PreProv.Edges[j].To = fmt.Sprintf("run_%d_pre_%s", m.Runs[i].Iteration, m.Runs[i].PreProv.Edges[j].To)
		}

		rawPostProvCont, err := ioutil.ReadFile(postProvFile)
		if err != nil {
			return fmt.Errorf("Failed reading consequent provenance of file '%v': %v", postProvFile, err)
		}

		err = json.Unmarshal(rawPostProvCont, &m.Runs[i].PostProv)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal JSON consequent provenance data: %v\n", err)
		}

		for j := range m.Runs[i].PostProv.Goals {

			if m.Runs[i].PostProv.Goals[j].Table == "clock" {

				clkTimeWildRegex := regexp.MustCompile(`, ([\d]+), __WILDCARD__\)`)
				clkTimeWildMatches := clkTimeWildRegex.FindStringSubmatch(m.Runs[i].PostProv.Goals[j].Label)

				clkTimeTwoRegex := regexp.MustCompile(`, ([\d]+), ([\d]+)\)`)
				clkTimeTwoMatches := clkTimeTwoRegex.FindStringSubmatch(m.Runs[i].PostProv.Goals[j].Label)

				if len(clkTimeWildMatches) > 0 {
					m.Runs[i].PostProv.Goals[j].Time = clkTimeWildMatches[1]
				}

				if len(clkTimeTwoMatches) > 0 {
					m.Runs[i].PostProv.Goals[j].Time = clkTimeTwoMatches[1]
				}
			}

			// Prefix goals with "post_".
			m.Runs[i].PostProv.Goals[j].ID = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Goals[j].ID)

			// Tentative mark as consequent not yet achieved
			// until we can do graph operations on this provenance.
			m.Runs[i].PostProv.Goals[j].CondHolds = false
		}

		// Prefix rules with "post_".
		for j := range m.Runs[i].PostProv.Rules {
			m.Runs[i].PostProv.Rules[j].ID = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Rules[j].ID)
		}

		// Prefix edges with "post_".
		for j := range m.Runs[i].PostProv.Edges {
			m.Runs[i].PostProv.Edges[j].From = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Edges[j].From)
			m.Runs[i].PostProv.Edges[j].To = fmt.Sprintf("run_%d_post_%s", m.Runs[i].Iteration, m.Runs[i].PostProv.Edges[j].To)
		}

		// Prepare slice for recommendations.
		m.Runs[i].Recommendation = make([]string, 0, 5)
	}

	return nil
}

// GetFailureSpec returns the failure specification of this analysis.
func (m *Molly) GetFailureSpec() *FailureSpec {
	return m.Runs[0].FailureSpec
}

// GetMsgsFailedRuns returns the messages sent from all failed runs.
func (m *Molly) GetMsgsFailedRuns() [][]*Message {

	msgs := make([][]*Message, len(m.FailedRunsIters))
	for i := range m.FailedRunsIters {
		msgs[i] = make([]*Message, len(m.Runs[m.FailedRunsIters[i]].Messages))
		msgs[i] = m.Runs[m.FailedRunsIters[i]].Messages
	}

	return msgs
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

// GetSuccessRunsIters returns indexes of successful runs.
func (m *Molly) GetSuccessRunsIters() []uint {
	return m.SuccessRunsIters
}

// GetFailedRunsIters returns indexes of failed runs.
func (m *Molly) GetFailedRunsIters() []uint {
	return m.FailedRunsIters
}
