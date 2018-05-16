package graphing

import (
	"fmt"
	"io"
	"strings"

	"os/exec"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Functions.

// CreateNaiveDiffProv
func (n *Neo4J) CreateNaiveDiffProv(symmetric bool, failedRuns []uint, successPostProv *gographviz.Graph) ([]*gographviz.Graph, []*gographviz.Graph, []*fi.Missing, error) {

	fmt.Printf("Creating differential provenance (good - bad), naive way...")

	exportQuery := `CALL apoc.export.cypher.query("
	MATCH (failed:Goal {run: ###RUN###, condition: 'post'})
	WITH collect(failed.label) AS failGoals

	MATCH pathSucc = (root:Goal {run: 0, condition: 'post'})-[*0..]->(goal:Goal {run: 0, condition: 'post'})
	WHERE NOT root.label IN failGoals AND NOT goal.label IN failGoals
	RETURN pathSucc;
	", "/tmp/export-differential-provenance", {format: "cypher-shell", cypherFormat: "create"})
	YIELD file, source, format, nodes, relationships, properties, time
	RETURN file, source, format, nodes, relationships, properties, time;`

	diffDots := make([]*gographviz.Graph, len(failedRuns))
	failedDots := make([]*gographviz.Graph, len(failedRuns))
	missingEvents := make([]*fi.Missing, len(failedRuns))

	for i := range failedRuns {

		diffRunID := 1000 + failedRuns[i]

		// Replace failed run in skeleton query.
		tmpExportQuery := strings.Replace(exportQuery, "###RUN###", fmt.Sprintf("%d", failedRuns[i]), -1)
		_, err := n.Conn1.ExecNeo(tmpExportQuery, nil)
		if err != nil {
			return nil, nil, nil, err
		}

		// Replace run ID part of node ID in saved queries.
		sedIDLong := fmt.Sprintf("s/`id`:\"run_0/`id`:\"run_%d/g", diffRunID)
		cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDLong, "/tmp/export-differential-provenance")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, nil, nil, err
		}

		if strings.TrimSpace(string(out)) != "" {
			return nil, nil, nil, fmt.Errorf("Wrong return value from docker-compose exec sed run ID command: %s", out)
		}

		// Replace run ID in saved queries.
		sedIDShort := fmt.Sprintf("s/`run`:0/`run`:%d/g", diffRunID)
		cmd = exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDShort, "/tmp/export-differential-provenance")
		out, err = cmd.CombinedOutput()
		if err != nil {
			return nil, nil, nil, err
		}

		if strings.TrimSpace(string(out)) != "" {
			return nil, nil, nil, fmt.Errorf("Wrong return value from docker-compose exec sed run ID command: %s", out)
		}

		// Import modified difference graph as new one.
		_, err = n.Conn1.ExecNeo(`
			CALL apoc.cypher.runFile("/tmp/export-differential-provenance", {statistics: false});
		`, nil)
		if err != nil {
			return nil, nil, nil, err
		}

		// Query differential provenance graph for leaves.
		stmtLeaves, err := n.Conn1.PrepareNeo(`
			MATCH path = (root:Goal {run: {run}, condition: "post"})-[*0..]->(:Rule {run: {run}, condition: "post"})-[*1]->(leaf:Goal {run: {run}, condition: "post"})
			WHERE NOT ()-->(root) AND NOT (leaf)-->()
			WITH length(path) AS maxLen
			ORDER BY maxLen DESC
			LIMIT 1
			WITH maxLen

			MATCH path = (root:Goal {run: {run}, condition: "post"})-[*0..]->(rule:Rule {run: {run}, condition: "post"})-[*1]->(leaf:Goal {run: {run}, condition: "post"})
			WHERE NOT ()-->(root) AND NOT (leaf)-->() AND length(path) = maxLen

			WITH DISTINCT rule
			MATCH (rule)-[*1]->(leaf:Goal {run: {run}, condition: "post"})
			WITH rule, collect(leaf) AS leaves

			RETURN rule, leaves;
		`)
		if err != nil {
			return nil, nil, nil, err
		}

		leavesRaw, err := stmtLeaves.QueryNeo(map[string]interface{}{
			"run": diffRunID,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		leavesAll, _, err := leavesRaw.All()
		if err != nil {
			return nil, nil, nil, err
		}

		rule := leavesAll[0][0].(graph.Node)
		missing := &fi.Missing{
			Rule: &fi.Rule{
				ID:    rule.Properties["id"].(string),
				Label: rule.Properties["label"].(string),
				Table: rule.Properties["table"].(string),
				Type:  rule.Properties["type"].(string),
			},
			Goals: make([]*fi.Goal, 0, 2),
		}

		// Add all leaves.
		leaves := leavesAll[0][1].([]interface{})
		for l := range leaves {

			leaf := leaves[l].(graph.Node)

			missing.Goals = append(missing.Goals, &fi.Goal{
				ID:        leaf.Properties["id"].(string),
				Label:     leaf.Properties["label"].(string),
				Table:     leaf.Properties["table"].(string),
				Time:      leaf.Properties["time"].(string),
				CondHolds: leaf.Properties["condition_holds"].(bool),
			})
		}

		err = leavesRaw.Close()
		if err != nil {
			return nil, nil, nil, err
		}

		err = stmtLeaves.Close()
		if err != nil {
			return nil, nil, nil, err
		}

		// Query for imported differential provenance.
		stmtProv, err := n.Conn1.PrepareNeo(`
			MATCH path = ({run: {run}, condition: "post"})-[:DUETO*1]->({run: {run}, condition: "post"})
			RETURN path;
		`)
		if err != nil {
			return nil, nil, nil, err
		}

		edgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
			"run": diffRunID,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		diffEdges := make([]graph.Path, 0, 10)

		for err == nil {

			var edgeRaw []interface{}

			edgeRaw, _, err = edgesRaw.NextNeo()
			if err != nil && err != io.EOF {
				return nil, nil, nil, err
			} else if err == nil {

				// Type-assert raw edge into well-defined struct.
				edge := edgeRaw[0].(graph.Path)

				// Append to slice of edges.
				diffEdges = append(diffEdges, edge)
			}
		}

		err = edgesRaw.Close()
		if err != nil {
			return nil, nil, nil, err
		}

		edgesRaw, err = stmtProv.QueryNeo(map[string]interface{}{
			"run": failedRuns[i],
		})
		if err != nil {
			return nil, nil, nil, err
		}

		failedEdges := make([]graph.Path, 0, 10)

		for err == nil {

			var edgeRaw []interface{}

			edgeRaw, _, err = edgesRaw.NextNeo()
			if err != nil && err != io.EOF {
				return nil, nil, nil, err
			} else if err == nil {

				// Type-assert raw edge into well-defined struct.
				edge := edgeRaw[0].(graph.Path)

				// Append to slice of edges.
				failedEdges = append(failedEdges, edge)
			}
		}

		// Pass to DOT string generator.
		diffDot, failedDot, err := createDiffDot(diffRunID, diffEdges, failedRuns[i], failedEdges, 0, successPostProv, missing)
		if err != nil {
			return nil, nil, nil, err
		}

		err = stmtProv.Close()
		if err != nil {
			return nil, nil, nil, err
		}

		diffDots[i] = diffDot
		failedDots[i] = failedDot
		missingEvents[i] = missing
	}

	fmt.Printf(" done\n\n")

	return diffDots, failedDots, missingEvents, nil
}
