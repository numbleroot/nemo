package graphing

import (
	"fmt"
	"io"
	"strings"
	"time"

	"os/exec"

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

// InitGraphDB
func (n *Neo4J) InitGraphDB(boltURI string) error {

	// Run the docker start command.
	fmt.Printf("Starting docker containers...")
	cmd := exec.Command("sudo", "docker-compose", "-f", "docker-compose.yml", "up", "-d")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if !strings.Contains(string(out), "done") {
		return fmt.Errorf("Wrong return value from docker-compose up command: %s", out)
	}
	fmt.Printf(" done\n")

	// Wait long enough for graph database to be up.
	time.Sleep(5 * time.Second)

	driver := neo4j.NewDriver()

	// Connect to bolt endpoint.
	c1, err := driver.OpenNeo(boltURI)
	if err != nil {
		return err
	}

	c2, err := driver.OpenNeo(boltURI)
	if err != nil {
		return err
	}

	n.Conn1 = c1
	n.Conn2 = c2

	fmt.Println()

	return nil
}

// CloseDB properly shuts down the Neo4J connection.
func (n *Neo4J) CloseDB() error {

	err := n.Conn1.Close()
	if err != nil {
		return err
	}

	err = n.Conn2.Close()
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	// Shut down docker container.
	fmt.Printf("Shutting down docker containers...")
	cmd := exec.Command("sudo", "docker-compose", "-f", "docker-compose.yml", "down")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if !strings.Contains(string(out), "done") {
		return fmt.Errorf("Wrong return value from docker-compose down command: %s", out)
	}
	fmt.Printf(" done\n")

	return nil
}

func (n *Neo4J) loadProv(iteration uint, provCond string, provData *faultinjectors.ProvData) error {

	stmtGoal, err := n.Conn1.PrepareNeo("CREATE (goal:Goal {id: {id}, run: {run}, condition: {condition}, label: {label}, table: {table}, type: {type}});")
	if err != nil {
		return err
	}

	var resCnt int64 = 0

	for j := range provData.Goals {

		// Create a goal node.
		res, err := stmtGoal.ExecNeo(map[string]interface{}{
			"id":        provData.Goals[j].ID,
			"run":       iteration,
			"condition": provCond,
			"label":     provData.Goals[j].Label,
			"table":     provData.Goals[j].Table,
			"type":      "single",
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
			"type":      "single",
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

// CreateNaiveDiffProv
func (n *Neo4J) CreateNaiveDiffProv(symmetric bool, failedRuns []uint) ([]string, error) {

	exportQuery := "CALL apoc.export.cypher.query(\"MATCH (failed:Goal {run: ###RUN###, condition: 'post'}) WITH collect(failed.label) AS failGoals MATCH pathSucc = (root:Goal {run: 0, condition: 'post'})-[*0..]->(goal:Goal {run: 0, condition: 'post'}) WHERE NOT root.label IN failGoals AND NOT goal.label IN failGoals RETURN pathSucc;\", \"/tmp/export-differential-provenance\", {format:\"plain\",cypherFormat:\"create\"}) YIELD file, source, format, nodes, relationships, properties, time RETURN file, source, format, nodes, relationships, properties, time;"

	dotStrings := make([]string, len(failedRuns))

	for i := range failedRuns {

		diffRunID := 1000 + failedRuns[i]

		// Replace failed run in skeleton query.
		tmpExportQuery := strings.Replace(exportQuery, "###RUN###", fmt.Sprintf("%d", failedRuns[i]), -1)
		_, err := n.Conn1.ExecNeo(tmpExportQuery, nil)
		if err != nil {
			return nil, err
		}

		// Replace run ID part of node ID in saved queries.
		sedIDLong := fmt.Sprintf("s/`id`:\"run_0/`id`:\"run_%d/g", diffRunID)
		cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDLong, "/tmp/export-differential-provenance")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(string(out)) != "" {
			return nil, fmt.Errorf("Wrong return value from docker-compose exec sed run ID command: %s", out)
		}

		// Replace run ID in saved queries.
		sedIDShort := fmt.Sprintf("s/`run`:0/`run`:%d/g", diffRunID)
		cmd = exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDShort, "/tmp/export-differential-provenance")
		out, err = cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(string(out)) != "" {
			return nil, fmt.Errorf("Wrong return value from docker-compose exec sed run ID command: %s", out)
		}

		// Import modified difference graph as new one.
		_, err = n.Conn1.ExecNeo("CALL apoc.cypher.runFile(\"/tmp/export-differential-provenance\")", nil)
		if err != nil {
			return nil, err
		}

		// Query for imported differential provenance.
		stmtDiff, err := n.Conn1.PrepareNeo("MATCH path = ({run: {run}})-[:DUETO*1]->({run: {run}}) RETURN path;")
		if err != nil {
			return nil, err
		}

		edgesRaw, err := stmtDiff.QueryNeo(map[string]interface{}{
			"run": diffRunID,
		})
		if err != nil {
			return nil, err
		}

		edges := make([]graph.Path, 0, 10)

		for err == nil {

			var edgeRaw []interface{}

			edgeRaw, _, err = edgesRaw.NextNeo()
			if err != nil && err != io.EOF {
				return nil, err
			} else if err == nil {

				// Type-assert raw edge into well-defined struct.
				edge := edgeRaw[0].(graph.Path)

				// Append to slice of edges.
				edges = append(edges, edge)
			}
		}

		// Pass to DOT string generator.
		dotString, err := createDOT(edges)
		if err != nil {
			return nil, err
		}

		err = stmtDiff.Close()
		if err != nil {
			return nil, err
		}

		dotStrings[i] = dotString
	}

	return dotStrings, nil
}
