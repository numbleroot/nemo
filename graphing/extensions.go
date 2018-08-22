package graphing

import (
	"fmt"
	"io"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Functions.

// GenerateExtensions
func (n *Neo4J) GenerateExtensions() (bool, []string, error) {

	// Track if all runs achieve the antecedent.
	allAchievedPre := true

	// Prepare slice of extensions.
	extensions := make([]string, 0, 3)

	// Prepare map for adding extensions only once per rule.
	rulesState := make(map[string]string)

	// Query for antecedent achievement per run.
	preAchievedRows, err := n.Conn1.QueryNeo(`
		MATCH (pre:Goal {condition: "pre", table: "pre", condition_holds: true})
		WHERE pre.run < 1000
		RETURN collect(pre) AS pres;
	`, nil)
	if err != nil {
		return false, nil, err
	}

	var preAchievedRaw []interface{}

	preAchievedRaw, _, err = preAchievedRows.NextNeo()
	if err != nil && err != io.EOF {
		return false, nil, err
	} else if err == nil {

		// Collect actual result we are interested in.
		preAchieved := preAchievedRaw[0].([]interface{})

		// Only in case this slice has as many members as
		// our execution has runs, we do not switch the
		// allAchievedPre flag to false.
		if len(preAchieved) < len(n.Runs) {
			allAchievedPre = false
		}
	}

	err = preAchievedRows.Close()
	if err != nil {
		return false, nil, err
	}

	if !allAchievedPre {

		// In case not all runs achieved the antecedent,
		// we query the successful (first) run and collect
		// all network events.

		asyncEventsRows, err := n.Conn1.QueryNeo(`
			MATCH (r:Rule {run: 0, condition: "pre", type: "async"})
			WHERE (:Goal {run: 0, condition: "pre", condition_holds: true})-[*1]->(r)-[*1]->(:Goal {run: 0, condition: "pre", condition_holds: false})-[*1]->(:Rule {run: 0, condition: "pre"}) OR (:Goal {run: 0, condition: "pre", condition_holds: false})-[*1]->(r)
			RETURN r;
		`, nil)
		if err != nil {
			return false, nil, err
		}

		asyncEventsRaw, _, err := asyncEventsRows.All()
		if err != nil {
			return false, nil, err
		}

		for i := range asyncEventsRaw {

			rule := asyncEventsRaw[i][0].(graph.Node)

			// Add rule to extension suggestions only
			// in case we did not already do so.
			rulesState[rule.Properties["table"].(string)] = fmt.Sprintf("<code>%s(node, ...)@async :- ...;</code>", rule.Properties["table"].(string))
		}

		for rule := range rulesState {

			// Append an extension suggestion to the final slice.
			extensions = append(extensions, rulesState[rule])
		}

		err = asyncEventsRows.Close()
		if err != nil {
			return false, nil, err
		}
	}

	return allAchievedPre, extensions, nil
}
