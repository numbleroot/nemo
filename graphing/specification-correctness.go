package graphing

import (
	"fmt"
	"io"
	"strings"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Functions.

// findAsyncEvents
func (n *Neo4J) findAsyncEvents(failedRun uint) ([]graph.Node, []graph.Node, error) {

	diffRunID := 1000 + failedRun

	// Determine if there is non-triviality (i.e., async events)
	// in the failed run's precondition provenance.
	stmtPreAsync, err := n.Conn2.PrepareNeo(`
        MATCH (r:Rule {run: {run}, condition: "pre", type: "async"})
        RETURN r;
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

	preAsyncs := make([]graph.Node, 0, 5)

	for err == nil {

		var preAsync []interface{}

		preAsync, _, err = preAsyncsRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, nil, err
		} else if err == nil {

			// Type-assert raw node into well-defined struct.
			node := preAsync[0].(graph.Node)

			// Provide raw name excluding "_provX" ending.
			labelParts := strings.Split(node.Properties["label"].(string), "_")
			node.Properties["raw_label"] = strings.Join(labelParts[:(len(labelParts)-1)], "_")

			// Append to slice of nodes.
			preAsyncs = append(preAsyncs, node)
		}
	}

	err = preAsyncsRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	// Determine if there is non-triviality (i.e., async events)
	// in differential postcondition provenance.
	stmtDiffAsync, err := n.Conn1.PrepareNeo(`
        MATCH (r:Rule {run: {run}, condition: "post", type: "async"})
        RETURN r;
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

	diffAsyncs := make([]graph.Node, 0, 5)

	for err == nil {

		var diffAsync []interface{}

		diffAsync, _, err = diffAsyncsRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, nil, err
		} else if err == nil {

			// Type-assert raw node into well-defined struct.
			node := diffAsync[0].(graph.Node)

			// Provide raw name excluding "_provX" ending.
			labelParts := strings.Split(node.Properties["label"].(string), "_")
			node.Properties["raw_label"] = strings.Join(labelParts[:(len(labelParts)-1)], "_")

			// Append to slice of nodes.
			diffAsyncs = append(diffAsyncs, node)
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

			// TODO: Clean up print-outs of rules (get rid of '_provX' at the end).
			preAsyncsLabel := fmt.Sprintf("<code>%s</code>", preAsyncs[0].Properties["raw_label"])
			for j := 1; j < len(preAsyncs); j++ {
				preAsyncsLabel = fmt.Sprintf("%s, <code>%s</code>", preAsyncsLabel, preAsyncs[j].Properties["raw_label"])
			}

			if len(diffAsyncs) < 1 {
				// No message passing events in differential postcondition provenance.
				corrections = append(corrections, fmt.Sprintf("%d message passing event(s) required for achieving precondition: %s", len(preAsyncs), preAsyncsLabel))
				corrections = append(corrections, "No message passing event left required for achieving postcondition.")
				corrections = append(corrections, "Yet we saw a fault occuring. Discuss: What are the use cases?")
			} else {

				diffAsyncsLabel := fmt.Sprintf("<code>%s</code>", diffAsyncs[0].Properties["raw_label"])
				for j := 1; j < len(diffAsyncs); j++ {
					diffAsyncsLabel = fmt.Sprintf("%s, <code>%s</code>", diffAsyncsLabel, diffAsyncs[j].Properties["raw_label"])
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
