package graphing

import (
	"fmt"
	"strings"

	"os/exec"
)

// CreatePrototype
func (n *Neo4J) CreatePrototype(iters []uint) error {

	fmt.Printf("Running extraction of success prototype...")

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
			"condition": "post",
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

	exportQuery := `CALL apoc.export.cypher.query("
	MATCH path = (r:Goal {run: 0, condition: 'post'})-[*0..]->(l:Goal {run: 0, condition: 'post'})
	WHERE r.label IN ###PROTOTYPE### AND l.label IN ###PROTOTYPE###
	RETURN path;
	", "/tmp/export-prototype-post", {format: "cypher-shell", cypherFormat: "create"})
	YIELD file, source, format, nodes, relationships, properties, time
	RETURN file, source, format, nodes, relationships, properties, time;`

	tmpExportQuery := strings.Replace(exportQuery, "###PROTOTYPE###", protoProv, -1)
	_, err = n.Conn1.ExecNeo(tmpExportQuery, nil)
	if err != nil {
		return err
	}

	// Replace run ID part of node ID in saved queries.
	cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", "s/`id`:\"run_0/`id`:\"run_2000/g", "/tmp/export-prototype-post")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed prototype run ID command: %s", out)
	}

	// Replace run ID in saved queries.
	cmd = exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", "s/`run`:0/`run`:2000/g", "/tmp/export-prototype-post")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed prototype run ID command: %s", out)
	}

	// Import modified prototype graph as new one.
	_, err = n.Conn1.ExecNeo(`
		CALL apoc.cypher.runFile("/tmp/export-prototype-post", {statistics: false});
	`, nil)
	if err != nil {
		return err
	}

	fmt.Printf(" done\n\n")

	return nil
}
