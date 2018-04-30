package graphing

import (
	"fmt"
	"io"
	"strings"

	"os/exec"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Functions.

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
