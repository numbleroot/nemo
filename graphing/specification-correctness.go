package graphing

import (
	"fmt"
	"io"

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

			if len(diffAsyncs) < 1 {
				corrections = append(corrections, "No message passing event required for achieving precondition and none left that has not already been taken place for achieving the postcondition. We still experience a fault. What are the use cases?")
			} else {
				corrections = append(corrections, "Achieving the precondition does not seem to depend on any message passing event. There are, though, message passing events missing that prevent the postcondition from being achieved. Introduce more fault-tolerance through replication and retries.")
			}
		} else {

			if len(diffAsyncs) < 1 {

			} else {

			}
		}

		allCorrections[i] = corrections
		allPrePostPairs[i] = prePostPairs
	}

	return allCorrections, allPrePostPairs, nil
}
