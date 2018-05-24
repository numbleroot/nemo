package graphing

import (
	"fmt"
	"strings"

	"os/exec"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// CreatePrototype
func (n *Neo4J) CreatePrototype(iters []uint) (*gographviz.Graph, error) {

	fmt.Printf("Running extraction of success prototype...")

	stmtCondGoals, err := n.Conn1.PrepareNeo(`
        MATCH (g1:Goal {run: {run}, condition: {condition}})
        OPTIONAL MATCH (g2:Goal {run: {run}, condition: "pre", condition_holds: true})
        WITH g1, collect(g2) AS existsSuccess
        WHERE size(existsSuccess) > 0
        RETURN collect(g1.label) AS goals;
    `)
	if err != nil {
		return nil, err
	}

	var protoProv string

	achvdCond := 0
	allProv := make(map[string]int)
	iterProv := make([]map[string]bool, len(iters))

	for i := range iters {

		// Request all goal labels as long as the
		// execution eventually achieved its condition.
		condGoals, err := stmtCondGoals.QueryNeo(map[string]interface{}{
			"run":       iters[i],
			"condition": "post",
		})
		if err != nil {
			return nil, err
		}

		condGoalsAll, _, err := condGoals.All()
		if err != nil {
			return nil, err
		}

		err = condGoals.Close()
		if err != nil {
			return nil, err
		}

		iterProv[i] = make(map[string]bool)

		for j := range condGoalsAll {

			for k := range condGoalsAll[j] {

				labels := condGoalsAll[j][k].([]interface{})

				if len(labels) > 0 {
					achvdCond += 1
				}

				for l := range labels {

					label := labels[l].(string)

					allProv[label] += 1
					iterProv[i][label] = true
				}
			}
		}
	}

	for label := range allProv {

		if allProv[label] == achvdCond {

			// Label is present in all label sets.
			// Add it to final (intersection) prototype.
			if protoProv == "" {
				protoProv = fmt.Sprintf("['%s'", label)
			} else {
				protoProv = fmt.Sprintf("%s, '%s'", protoProv, label)
			}
		}
	}

	// Finish list.
	protoProv = fmt.Sprintf("%s]", protoProv)

	err = stmtCondGoals.Close()
	if err != nil {
		return nil, err
	}

	exportQuery := `CALL apoc.export.cypher.query("
	MATCH path = (r:Goal {run: 0, condition: 'post'})-[*1]->(Rule)-[*1]->(l:Goal {run: 0, condition: 'post'})
	WHERE r.label IN ###PROTOTYPE### AND l.label IN ###PROTOTYPE###
	RETURN path;
	", "/tmp/export-prototype-post", {format: "cypher-shell", cypherFormat: "create"})
	YIELD file, source, format, nodes, relationships, properties, time
	RETURN file, source, format, nodes, relationships, properties, time;`

	tmpExportQuery := strings.Replace(exportQuery, "###PROTOTYPE###", protoProv, -1)
	_, err = n.Conn1.ExecNeo(tmpExportQuery, nil)
	if err != nil {
		return nil, err
	}

	// Replace run ID part of node ID in saved queries.
	cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", "s/`id`:\"run_0/`id`:\"run_2000/g", "/tmp/export-prototype-post")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(string(out)) != "" {
		return nil, fmt.Errorf("Wrong return value from docker-compose exec sed prototype run ID command: %s", out)
	}

	// Replace run ID in saved queries.
	cmd = exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", "s/`run`:0/`run`:2000/g", "/tmp/export-prototype-post")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(string(out)) != "" {
		return nil, fmt.Errorf("Wrong return value from docker-compose exec sed prototype run ID command: %s", out)
	}

	// Import modified prototype graph as new one.
	_, err = n.Conn1.ExecNeo(`
		CALL apoc.cypher.runFile("/tmp/export-prototype-post", {statistics: false});
	`, nil)
	if err != nil {
		return nil, err
	}

	// Query for imported intersection prototype provenance.
	stmtProv, err := n.Conn1.PrepareNeo(`
		MATCH path = ({run: 2000, condition: "post"})-[:DUETO*1]->({run: 2000, condition: "post"})
		RETURN path;
	`)
	if err != nil {
		return nil, err
	}

	edges := make([]graph.Path, 0, 20)

	edgesRaw, err := stmtProv.QueryNeo(nil)
	if err != nil {
		return nil, err
	}

	edgesRows, _, err := edgesRaw.All()
	if err != nil {
		return nil, err
	}

	for r := range edgesRows {

		// Type-assert raw edge into well-defined struct.
		edge := edgesRows[r][0].(graph.Path)

		// Append to slice of edges.
		edges = append(edges, edge)
	}

	// Pass to DOT string generator.
	prototypeDot, err := createDOT(edges, "prototype")
	if err != nil {
		return nil, err
	}

	err = edgesRaw.Close()
	if err != nil {
		return nil, err
	}

	err = stmtProv.Close()
	if err != nil {
		return nil, err
	}

	fmt.Printf(" done\n\n")

	return prototypeDot, nil
}
