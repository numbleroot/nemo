package graphing

import (
	"fmt"
	"io"
	"strings"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Functions.

// GenerateExtensions
func (n *Neo4J) GenerateExtensions() ([]string, error) {

	// Track if all runs achieve the precondition.
	allAchievedPre := true

	// Prepare slice of extensions.
	extensions := make([]string, 0, 3)

	// Query for precondition achievement per run.
	preAchievedRows, err := n.Conn1.QueryNeo(`
		MATCH (pre:Goal {condition: "pre", table: "pre", condition_holds: true})
		WHERE pre.run < 1000
		RETURN collect(pre) AS pres;
	`, nil)
	if err != nil {
		return nil, err
	}

	var preAchievedRaw []interface{}

	preAchievedRaw, _, err = preAchievedRows.NextNeo()
	if err != nil && err != io.EOF {
		return nil, err
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
		return nil, err
	}

	if !allAchievedPre {

		// In case not all runs achieved the precondition,
		// we query the successful (first) run and collect
		// all network events.

		asyncEventsRows, err := n.Conn1.QueryNeo(`
			MATCH (g:Goal {run: 0, condition: "pre"})-[*1]->(r:Rule {run: 0, condition: "pre", type: "async"})
			WHERE (g.condition_holds = true AND (g)-[*1]->(r)-[*1]->(:Goal {run: 0, condition: "pre", condition_holds: false})-[*1]->(:Rule {run: 0, condition: "pre"}))
			      OR g.condition_holds = false
			RETURN g, r;
		`, nil)
		if err != nil {
			return nil, err
		}

		asyncEventsRaw, _, err := asyncEventsRows.All()
		if err != nil {
			return nil, err
		}

		for i := range asyncEventsRaw {

			goal := asyncEventsRaw[i][0].(graph.Node)
			rule := asyncEventsRaw[i][1].(graph.Node)

			// Parse parts that make up label of goal.
			goalLabel := strings.TrimLeft(goal.Properties["label"].(string), goal.Properties["table"].(string))
			goalLabel = strings.Trim(goalLabel, "()")
			goalLabelParts := strings.Split(goalLabel, ", ")

			// Append an extension suggestion to the final slice.
			extensions = append(extensions, fmt.Sprintf("<code>%s(%s, ...)@async :- ...;</code>", rule.Properties["table"].(string), goalLabelParts[0]))
		}

		err = asyncEventsRows.Close()
		if err != nil {
			return nil, err
		}
	}

	return extensions, nil
}
