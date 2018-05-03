package graphing

import (
	"fmt"
	"strings"

	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	"github.com/numbleroot/nemo/faultinjectors"
)

// Structs.

// Neo4J
type Neo4J struct {
	Conn1 neo4j.Conn
	Conn2 neo4j.Conn
}

// Functions.

// loadProv
func (n *Neo4J) loadProv(iteration uint, provCond string, provData *faultinjectors.ProvData) error {

	stmtGoal, err := n.Conn1.PrepareNeo("CREATE (goal:Goal {id: {id}, run: {run}, condition: {condition}, label: {label}, table: {table}, time: {time}, condition_holds: {condition_holds}});")
	if err != nil {
		return err
	}

	var resCnt int64 = 0

	for j := range provData.Goals {

		// Create a goal node.
		res, err := stmtGoal.ExecNeo(map[string]interface{}{
			"id":              provData.Goals[j].ID,
			"run":             iteration,
			"condition":       provCond,
			"label":           provData.Goals[j].Label,
			"table":           provData.Goals[j].Table,
			"time":            provData.Goals[j].Time,
			"condition_holds": provData.Goals[j].CondHolds,
		})
		if err != nil {
			return err
		}

		// Collect affected rows information.
		rowsAff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		resCnt += rowsAff
	}

	err = stmtGoal.Close()
	if err != nil {
		return err
	}

	// During first run: create constraints and indexes.
	if iteration == 0 {

		_, err = n.Conn1.ExecNeo("CREATE CONSTRAINT ON (goal:Goal) ASSERT goal.id IS UNIQUE;", nil)
		if err != nil {
			return err
		}

		_, err = n.Conn1.ExecNeo("CREATE INDEX ON :Goal(run);", nil)
		if err != nil {
			return err
		}
	}

	// Verify number of inserted elements.
	if int64(len(provData.Goals)) != resCnt {
		return fmt.Errorf("Run %d: inserted number of goals (%d) does not equal number of precondition provenance goals (%d)", iteration, resCnt, len(provData.Goals))
	}

	resCnt = 0

	stmtRule, err := n.Conn1.PrepareNeo("CREATE (n:Rule {id: {id}, run: {run}, condition: {condition}, label: {label}, table: {table}, type: {type}});")
	if err != nil {
		return err
	}

	for j := range provData.Rules {

		// Create a rule node.
		res, err := stmtRule.ExecNeo(map[string]interface{}{
			"id":        provData.Rules[j].ID,
			"run":       iteration,
			"condition": provCond,
			"label":     provData.Rules[j].Label,
			"table":     provData.Rules[j].Table,
			"type":      provData.Rules[j].Type,
		})
		if err != nil {
			return err
		}

		// Collect affected rows information.
		rowsAff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		resCnt += rowsAff
	}

	err = stmtRule.Close()
	if err != nil {
		return err
	}

	// During first run: create constraints and indexes.
	if iteration == 0 {

		_, err = n.Conn1.ExecNeo("CREATE CONSTRAINT ON (rule:Rule) ASSERT rule.id IS UNIQUE;", nil)
		if err != nil {
			return err
		}

		_, err = n.Conn1.ExecNeo("CREATE INDEX ON :Rule(run);", nil)
		if err != nil {
			return err
		}
	}

	// Verify number of inserted elements.
	if int64(len(provData.Rules)) != resCnt {
		return fmt.Errorf("Run %d: inserted number of rules (%d) does not equal number of precondition provenance rules (%d)", iteration, resCnt, len(provData.Rules))
	}

	resCnt = 0

	stmtGoalRuleEdge, err := n.Conn1.PrepareNeo("MATCH (goal:Goal {id: {from}, run: {run}, condition: {condition}}) MATCH (rule:Rule {id: {to}, run: {run}, condition: {condition}}) MERGE (goal)-[:DUETO]->(rule);")
	if err != nil {
		return err
	}

	stmtRuleGoalEdge, err := n.Conn2.PrepareNeo("MATCH (rule:Rule {id: {from}, run: {run}, condition: {condition}}) MATCH (goal:Goal {id: {to}, run: {run}, condition: {condition}}) MERGE (rule)-[:DUETO]->(goal);")
	if err != nil {
		return err
	}

	for j := range provData.Edges {

		var res neo4j.Result

		// Create an edge relation.
		if strings.Contains(provData.Edges[j].From, "goal") {
			res, err = stmtGoalRuleEdge.ExecNeo(map[string]interface{}{
				"from":      provData.Edges[j].From,
				"to":        provData.Edges[j].To,
				"run":       iteration,
				"condition": provCond,
			})
		} else {
			res, err = stmtRuleGoalEdge.ExecNeo(map[string]interface{}{
				"from":      provData.Edges[j].From,
				"to":        provData.Edges[j].To,
				"run":       iteration,
				"condition": provCond,
			})
		}
		if err != nil {
			return err
		}

		// Track number of created relationships.
		stats := res.Metadata()["stats"].(map[string]interface{})
		resCnt += stats["relationships-created"].(int64)
	}

	err = stmtGoalRuleEdge.Close()
	if err != nil {
		return err
	}

	err = stmtRuleGoalEdge.Close()
	if err != nil {
		return err
	}

	// Verify number of inserted elements.
	if int64(len(provData.Edges)) != resCnt {
		return fmt.Errorf("Run %d: inserted number of edges (%d) does not equal number of precondition provenance edges (%d)", iteration, resCnt, len(provData.Edges))
	}

	return nil
}

// LoadNaiveProv
func (n *Neo4J) LoadNaiveProv(runs []*faultinjectors.Run) error {

	fmt.Printf("Loading provenance data (naive approach)...\n")

	for i := range runs {

		// Load precondition provenance.
		fmt.Printf("\t[%d] Precondition provenance...", runs[i].Iteration)
		err := n.loadProv(runs[i].Iteration, "pre", runs[i].PreProv)
		if err != nil {
			return err
		}
		fmt.Printf(" done\n")

		// Load postcondition provenance.
		fmt.Printf("\t[%d] Postcondition provenance...", runs[i].Iteration)
		err = n.loadProv(runs[i].Iteration, "post", runs[i].PostProv)
		if err != nil {
			return err
		}
		fmt.Printf(" done\n")
	}

	fmt.Println()

	return nil
}

// PullPrePostProv
func (n *Neo4J) PullPrePostProv(runs []*faultinjectors.Run) ([]string, []string, error) {

	preDotStrings := make([]string, len(runs))
	postDotStrings := make([]string, len(runs))

	// Query for imported correctness condition provenance.
	stmtProv, err := n.Conn1.PrepareNeo("MATCH path = ({run: {run}, condition: {condition}})-[:DUETO*1]->({run: {run}, condition: {condition}}) RETURN path;")
	if err != nil {
		return nil, nil, err
	}

	for i := range runs {

		preEdges := make([]graph.Path, 0, 20)
		postEdges := make([]graph.Path, 0, 20)

		preEdgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
			"run":       runs[i].Iteration,
			"condition": "pre",
		})
		if err != nil {
			return nil, nil, err
		}

		preEdgesRows, _, err := preEdgesRaw.All()
		if err != nil {
			return nil, nil, err
		}

		for p := range preEdgesRows {

			// Type-assert raw edge into well-defined struct.
			edge := preEdgesRows[p][0].(graph.Path)

			// Append to slice of edges.
			preEdges = append(preEdges, edge)
		}

		// Pass to DOT string generator.
		preDotString, err := createDOT(preEdges)
		if err != nil {
			return nil, nil, err
		}

		err = preEdgesRaw.Close()
		if err != nil {
			return nil, nil, err
		}

		postEdgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
			"run":       runs[i].Iteration,
			"condition": "post",
		})
		if err != nil {
			return nil, nil, err
		}

		postEdgesRows, _, err := postEdgesRaw.All()
		if err != nil {
			return nil, nil, err
		}

		for p := range postEdgesRows {

			// Type-assert raw edge into well-defined struct.
			edge := postEdgesRows[p][0].(graph.Path)

			// Append to slice of edges.
			postEdges = append(postEdges, edge)
		}

		// Pass to DOT string generator.
		postDotString, err := createDOT(postEdges)
		if err != nil {
			return nil, nil, err
		}

		err = postEdgesRaw.Close()
		if err != nil {
			return nil, nil, err
		}

		preDotStrings[i] = preDotString
		postDotStrings[i] = postDotString
	}

	err = stmtProv.Close()
	if err != nil {
		return nil, nil, err
	}

	return preDotStrings, postDotStrings, nil
}
