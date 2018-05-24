package graphing

import (
	"fmt"
	"strings"

	"github.com/awalterschulze/gographviz"
	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Structs.

// Neo4J
type Neo4J struct {
	Conn1 neo4j.Conn
	Conn2 neo4j.Conn
	Runs  []*fi.Run
}

// Functions.

// loadProv
func (n *Neo4J) loadProv(iteration uint, provCond string, provData *fi.ProvData) error {

	stmtGoal, err := n.Conn1.PrepareNeo(`
		CREATE (goal:Goal {id: {id}, run: {run}, condition: {condition}, label: {label}, table: {table}, time: {time}, condition_holds: {condition_holds}});
	`)
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

		_, err = n.Conn1.ExecNeo(`
			CREATE CONSTRAINT ON (goal:Goal) ASSERT goal.id IS UNIQUE;
		`, nil)
		if err != nil {
			return err
		}

		_, err = n.Conn1.ExecNeo(`
			CREATE INDEX ON :Goal(run);
		`, nil)
		if err != nil {
			return err
		}
	}

	// Verify number of inserted elements.
	if int64(len(provData.Goals)) != resCnt {
		return fmt.Errorf("Run %d: inserted number of goals (%d) does not equal number of precondition provenance goals (%d)", iteration, resCnt, len(provData.Goals))
	}

	resCnt = 0

	stmtRule, err := n.Conn1.PrepareNeo(`
		CREATE (n:Rule {id: {id}, run: {run}, condition: {condition}, label: {label}, table: {table}, type: {type}});
	`)
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

		_, err = n.Conn1.ExecNeo(`
			CREATE CONSTRAINT ON (rule:Rule) ASSERT rule.id IS UNIQUE;
		`, nil)
		if err != nil {
			return err
		}

		_, err = n.Conn1.ExecNeo(`
			CREATE INDEX ON :Rule(run);
		`, nil)
		if err != nil {
			return err
		}
	}

	// Verify number of inserted elements.
	if int64(len(provData.Rules)) != resCnt {
		return fmt.Errorf("Run %d: inserted number of rules (%d) does not equal number of precondition provenance rules (%d)", iteration, resCnt, len(provData.Rules))
	}

	resCnt = 0

	stmtGoalRuleEdge, err := n.Conn1.PrepareNeo(`
		MATCH (goal:Goal {id: {from}, run: {run}, condition: {condition}})
		MATCH (rule:Rule {id: {to}, run: {run}, condition: {condition}})
		MERGE (goal)-[:DUETO]->(rule);
	`)
	if err != nil {
		return err
	}

	stmtRuleGoalEdge, err := n.Conn2.PrepareNeo(`
		MATCH (rule:Rule {id: {from}, run: {run}, condition: {condition}})
		MATCH (goal:Goal {id: {to}, run: {run}, condition: {condition}})
		MERGE (rule)-[:DUETO]->(goal);
	`)
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
func (n *Neo4J) LoadNaiveProv() error {

	fmt.Printf("Loading provenance data (naive approach)...\n")

	for i := range n.Runs {

		// Load precondition provenance.
		fmt.Printf("\t[%d] Precondition provenance... ", n.Runs[i].Iteration)
		err := n.loadProv(n.Runs[i].Iteration, "pre", n.Runs[i].PreProv)
		if err != nil {
			return err
		}
		fmt.Printf("done\n")

		// Load postcondition provenance.
		fmt.Printf("\t[%d] Postcondition provenance... ", n.Runs[i].Iteration)
		err = n.loadProv(n.Runs[i].Iteration, "post", n.Runs[i].PostProv)
		if err != nil {
			return err
		}
		fmt.Printf("done\n")
	}

	fmt.Println()

	return nil
}

// PullPrePostProv
func (n *Neo4J) PullPrePostProv() ([]*gographviz.Graph, []*gographviz.Graph, error) {

	fmt.Printf("Pulling pre- and postcondition provenance... ")

	preDots := make([]*gographviz.Graph, len(n.Runs))
	postDots := make([]*gographviz.Graph, len(n.Runs))

	// Query for imported correctness condition provenance.
	stmtProv, err := n.Conn1.PrepareNeo(`
		MATCH path = ({run: {run}, condition: {condition}})-[:DUETO*1]->({run: {run}, condition: {condition}})
		RETURN path;
	`)
	if err != nil {
		return nil, nil, err
	}

	for i := range n.Runs {

		preEdges := make([]graph.Path, 0, 20)
		postEdges := make([]graph.Path, 0, 20)

		preEdgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
			"run":       n.Runs[i].Iteration,
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
		preDot, err := createDOT(preEdges, "pre")
		if err != nil {
			return nil, nil, err
		}

		err = preEdgesRaw.Close()
		if err != nil {
			return nil, nil, err
		}

		postEdgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
			"run":       n.Runs[i].Iteration,
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
		postDot, err := createDOT(postEdges, "post")
		if err != nil {
			return nil, nil, err
		}

		err = postEdgesRaw.Close()
		if err != nil {
			return nil, nil, err
		}

		preDots[i] = preDot
		postDots[i] = postDot
	}

	err = stmtProv.Close()
	if err != nil {
		return nil, nil, err
	}

	fmt.Printf("done\n\n")

	return preDots, postDots, nil
}
