package graphing

import (
	"fmt"
	"io"
	"strings"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Structs.

// CorrectionsPair
type CorrectionsPair struct {
	Rule *fi.Rule
	Goal *fi.Goal
}

// Functions.

// findAsyncEvents
func (n *Neo4J) findAsyncEvents(failedRun uint) ([]*CorrectionsPair, []*CorrectionsPair, error) {

	diffRunID := 1000 + failedRun

	// Determine if there is non-triviality (i.e., async events)
	// in the failed run's precondition provenance.
	stmtPreAsync, err := n.Conn2.PrepareNeo(`
        MATCH path = (root {run: {run}, condition: "pre"})-[*0..]->(r1:Rule {run: {run}, condition: "pre", type: "async"})-[*0..]->(r2:Rule {run: {run}, condition: "pre", type: "async"})
		WHERE NOT ()-->(root)
		WITH r2, collect(r1) AS history

		MATCH (g:Goal)-[*1]->(r2)
		WITH r2, g, history
		RETURN r2 AS rule, g AS goal, filter(_ IN history WHERE size(history) = 1) AS history;
    `)
	if err != nil {
		return nil, nil, err
	}

	preAsyncsRaw, err := stmtPreAsync.QueryNeo(map[string]interface{}{
		"run": failedRun,
	})
	if err != nil {
		return nil, nil, err
	}

	preAsyncs := make([]*CorrectionsPair, 0, 5)

	for err == nil {

		var preAsync []interface{}

		preAsync, _, err = preAsyncsRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, nil, err
		} else if err == nil {

			history := preAsync[2].([]interface{})

			// We only consider results that are not subsumed
			// by other results.
			if len(history) == 1 {

				// Type-assert the two nodes into well-defined struct.
				rule := preAsync[0].(graph.Node)
				goal := preAsync[1].(graph.Node)

				// Provide raw name excluding "_provX" ending.
				labelParts := strings.Split(rule.Properties["label"].(string), "_")
				rule.Properties["label"] = strings.Join(labelParts[:(len(labelParts)-1)], "_")

				// Append to slice of correction pairs.
				preAsyncs = append(preAsyncs, &CorrectionsPair{
					Rule: &fi.Rule{
						ID:    rule.Properties["id"].(string),
						Label: rule.Properties["label"].(string),
						Table: rule.Properties["table"].(string),
						Type:  rule.Properties["type"].(string),
					},
					Goal: &fi.Goal{
						ID:        goal.Properties["id"].(string),
						Label:     goal.Properties["label"].(string),
						Table:     goal.Properties["table"].(string),
						Time:      goal.Properties["time"].(string),
						CondHolds: goal.Properties["condition_holds"].(bool),
					},
				})
			}
		}
	}

	err = preAsyncsRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	// Determine if there is non-triviality (i.e., async events)
	// in differential postcondition provenance.
	stmtDiffAsync, err := n.Conn1.PrepareNeo(`
        MATCH path = (root {run: {run}, condition: "post"})-[*0..]->(r1:Rule {run: {run}, condition: "post", type: "async"})-[*0..]->(r2:Rule {run: {run}, condition: "post", type: "async"})
		WHERE NOT ()-->(root)
		WITH r2, collect(r1) AS history

		MATCH (g:Goal)-[*1]->(r2)
		WITH r2, g, history
		RETURN r2 AS rule, g AS goal, filter(_ IN history WHERE size(history) = 1) AS history;
    `)
	if err != nil {
		return nil, nil, err
	}

	diffAsyncsRaw, err := stmtDiffAsync.QueryNeo(map[string]interface{}{
		"run": diffRunID,
	})
	if err != nil {
		return nil, nil, err
	}

	diffAsyncs := make([]*CorrectionsPair, 0, 5)

	for err == nil {

		var diffAsync []interface{}

		diffAsync, _, err = diffAsyncsRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, nil, err
		} else if err == nil {

			history := diffAsync[2].([]interface{})

			if len(history) == 1 {

				// Type-assert the two nodes into well-defined struct.
				rule := diffAsync[0].(graph.Node)
				goal := diffAsync[1].(graph.Node)

				// Provide raw name excluding "_provX" ending.
				labelParts := strings.Split(rule.Properties["label"].(string), "_")
				rule.Properties["label"] = strings.Join(labelParts[:(len(labelParts)-1)], "_")

				// Append to slice of correction pairs.
				diffAsyncs = append(diffAsyncs, &CorrectionsPair{
					Rule: &fi.Rule{
						ID:    rule.Properties["id"].(string),
						Label: rule.Properties["label"].(string),
						Table: rule.Properties["table"].(string),
						Type:  rule.Properties["type"].(string),
					},
					Goal: &fi.Goal{
						ID:        goal.Properties["id"].(string),
						Label:     goal.Properties["label"].(string),
						Table:     goal.Properties["table"].(string),
						Time:      goal.Properties["time"].(string),
						CondHolds: goal.Properties["condition_holds"].(bool),
					},
				})
			}
		}
	}

	err = diffAsyncsRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	return preAsyncs, diffAsyncs, nil
}

// GenerateCorrections
func (n *Neo4J) GenerateCorrections(failedRuns []uint) ([][]string, [][]*fi.Correction, error) {

	fmt.Printf("Running generation of suggestions for corrections (pre ~> post)...")

	// Prepare final slices to return.
	allCorrections := make([][]string, len(failedRuns))
	allPrePostPairs := make([][]*fi.Correction, len(failedRuns))

	for i := range failedRuns {

		// Create local slices.
		corrections := make([]string, 0, 6)
		prePostPairs := make([]*fi.Correction, 0, 8)

		// Retrieve non-trivial events in pre and diffprov post.
		preAsyncs, diffAsyncs, err := n.findAsyncEvents(failedRuns[i])
		if err != nil {
			return nil, nil, err
		}

		if len(preAsyncs) < 1 {

			// No message passing events in precondition provenance.

			if len(diffAsyncs) < 1 {
				// No message passing events in differential postcondition provenance.
				corrections = append(corrections, "No message passing event required for achieving precondition.")
				corrections = append(corrections, "No message passing event left required for achieving postcondition.")
				corrections = append(corrections, "Yet we saw a fault occuring. Discuss: What are the use cases?")
			} else {
				// At least one message passing event in differential postcondition provenance.
				corrections = append(corrections, "No message passing event required for achieving precondition.")
				corrections = append(corrections, "There exist message passing events missing that prevent achieving the postcondition.")
				corrections = append(corrections, "Suggestion: Introduce more fault-tolerance through replication and retries.")
			}
		} else {

			// At least one message passing event in precondition provenance.

			preAsyncsLabel := fmt.Sprintf("<code>%s</code>", preAsyncs[0].Rule.Label)
			for j := 1; j < len(preAsyncs); j++ {
				preAsyncsLabel = fmt.Sprintf("%s, <code>%s</code>", preAsyncsLabel, preAsyncs[j].Rule.Label)
			}

			if len(diffAsyncs) < 1 {
				// No message passing events in differential postcondition provenance.
				corrections = append(corrections, fmt.Sprintf("%d message passing event(s) required for achieving precondition: %s", len(preAsyncs), preAsyncsLabel))
				corrections = append(corrections, "No message passing event left required for achieving postcondition.")
				corrections = append(corrections, "Yet we saw a fault occuring. Discuss: What are the use cases?")
			} else {

				diffAsyncsLabel := fmt.Sprintf("<code>%s</code>", diffAsyncs[0].Rule.Label)
				for j := 1; j < len(diffAsyncs); j++ {
					diffAsyncsLabel = fmt.Sprintf("%s, <code>%s</code>", diffAsyncsLabel, diffAsyncs[j].Rule.Label)
				}

				// At least one message passing event in differential postcondition provenance.
				corrections = append(corrections, fmt.Sprintf("%d message passing event(s) required for achieving precondition: %s", len(preAsyncs), preAsyncsLabel))
				corrections = append(corrections, fmt.Sprintf("%d message passing event(s) left required for achieving postcondition: %s", len(diffAsyncs), diffAsyncsLabel))
				corrections = append(corrections, fmt.Sprintf("How can you change the program to semantically depend '%s' on '%s'?", preAsyncsLabel, diffAsyncsLabel))
				corrections = append(corrections, fmt.Sprintf("How can you make the firing of '%s' dependent on guaranteed prior firing of '%s'?", preAsyncsLabel, diffAsyncsLabel))
				corrections = append(corrections, "TODO: Graphical aids...")

				// TODO: This is the important area.
			}
		}

		if len(corrections) == 0 {
			corrections = append(corrections, "No correction suggestions to make!")
		}

		allCorrections[i] = corrections
		allPrePostPairs[i] = prePostPairs
	}

	fmt.Printf(" done\n\n")

	return allCorrections, allPrePostPairs, nil
}
