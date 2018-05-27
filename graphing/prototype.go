package graphing

import (
	"fmt"
	"strings"

	"os/exec"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

func (n *Neo4J) extractPrototype(iters []uint, condition string) error {

	// Find all labels of next chain goals.
	stmtCondGoals, err := n.Conn1.PrepareNeo(`
        MATCH (g1:Goal {run: {run}, condition: {condition}})
        OPTIONAL MATCH (g2:Goal {run: {run}, condition: "pre", condition_holds: true})
        WITH g1, collect(g2) AS existsSuccess
        WHERE size(existsSuccess) > 0
        RETURN collect(g1.label) AS goals;
    `)
	if err != nil {
		return err
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
			"condition": condition,
		})
		if err != nil {
			return err
		}

		condGoalsAll, _, err := condGoals.All()
		if err != nil {
			return err
		}

		err = condGoals.Close()
		if err != nil {
			return err
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
		return err
	}

	// Export elements of run 0 that go into the prototype.
	exportQuery := `CALL apoc.export.cypher.query("
	MATCH path = (r:Goal {run: 0, condition: '###CONDITION###'})-[*1]->(Rule)-[*1]->(l:Goal {run: 0, condition: '###CONDITION###'})
	WHERE r.label IN ###PROTOTYPE### AND l.label IN ###PROTOTYPE###
	RETURN path;
	", "/tmp/export-prototype", {format: "cypher-shell", cypherFormat: "create"})
	YIELD time
	RETURN time;`

	exportQuery = strings.Replace(exportQuery, "###PROTOTYPE###", protoProv, -1)
	exportQuery = strings.Replace(exportQuery, "###CONDITION###", condition, -1)
	_, err = n.Conn1.ExecNeo(exportQuery, nil)
	if err != nil {
		return err
	}

	// Replace run ID part of node ID in saved queries.
	cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", "s/`id`:\"run_0/`id`:\"run_3000/g", "/tmp/export-prototype")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed prototype run ID command: %s", out)
	}

	// Replace run ID in saved queries.
	cmd = exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", "s/`run`:0/`run`:3000/g", "/tmp/export-prototype")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed prototype run ID command: %s", out)
	}

	// Import modified prototype graph as new one.
	_, err = n.Conn1.ExecNeo(`
		CALL apoc.cypher.runFile("/tmp/export-prototype", {statistics: false});
	`, nil)
	if err != nil {
		return err
	}

	return nil
}

// CreatePrototype
func (n *Neo4J) CreatePrototype(iters []uint) (*gographviz.Graph, *gographviz.Graph, error) {

	fmt.Printf("Running extraction of success prototype... ")

	// Create precondition prototype.
	err := n.extractPrototype(iters, "pre")
	if err != nil {
		return nil, nil, err
	}

	// Create postcondition prototype.
	err = n.extractPrototype(iters, "post")
	if err != nil {
		return nil, nil, err
	}

	// Query for imported intersection prototype provenance.
	stmtProv, err := n.Conn1.PrepareNeo(`
		MATCH path = ({run: 3000, condition: {condition}})-[:DUETO*1]->({run: 3000, condition: {condition}})
		RETURN path;
	`)
	if err != nil {
		return nil, nil, err
	}

	preEdges := make([]graph.Path, 0, 20)
	postEdges := make([]graph.Path, 0, 20)

	preEdgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
		"condition": "pre",
	})
	if err != nil {
		return nil, nil, err
	}

	preEdgesRows, _, err := preEdgesRaw.All()
	if err != nil {
		return nil, nil, err
	}

	for r := range preEdgesRows {

		// Type-assert raw edge into well-defined struct.
		edge := preEdgesRows[r][0].(graph.Path)

		// Append to slice of edges.
		preEdges = append(preEdges, edge)
	}

	// Pass to DOT string generator.
	protoPreDot, err := createDOT(preEdges, "prototype")
	if err != nil {
		return nil, nil, err
	}

	err = preEdgesRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	postEdgesRaw, err := stmtProv.QueryNeo(map[string]interface{}{
		"condition": "post",
	})
	if err != nil {
		return nil, nil, err
	}

	postEdgesRows, _, err := postEdgesRaw.All()
	if err != nil {
		return nil, nil, err
	}

	for r := range postEdgesRows {

		// Type-assert raw edge into well-defined struct.
		edge := postEdgesRows[r][0].(graph.Path)

		// Append to slice of edges.
		postEdges = append(postEdges, edge)
	}

	// Pass to DOT string generator.
	protoPostDot, err := createDOT(postEdges, "prototype")
	if err != nil {
		return nil, nil, err
	}

	err = postEdgesRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	err = stmtProv.Close()
	if err != nil {
		return nil, nil, err
	}

	fmt.Printf("done\n\n")

	return protoPreDot, protoPostDot, nil
}
