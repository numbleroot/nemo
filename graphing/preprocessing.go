package graphing

import (
	"fmt"
	"strings"

	"os/exec"
)

// cleanCopyProv
func (n *Neo4J) cleanCopyProv(iter uint, condition string) error {

	newID := 1000 + iter

	exportQuery := `CALL apoc.export.cypher.query("
	MATCH path = (g1:Goal {run: ###RUN###, condition: '###CONDITION###'})-[*0..]->(g2:Goal {run: ###RUN###, condition: '###CONDITION###'})
	RETURN path;
	", "/tmp/clean-prov", {format: "cypher-shell", cypherFormat: "create"})
	YIELD file, source, format, nodes, relationships, properties, time
	RETURN file, source, format, nodes, relationships, properties, time;`

	tmpExportQuery := strings.Replace(exportQuery, "###RUN###", fmt.Sprintf("%d", iter), -1)
	tmpExportQuery = strings.Replace(tmpExportQuery, "###CONDITION###", condition, -1)
	fmt.Printf("NEW EXPORT:\n'%#v'\n\n", tmpExportQuery)
	_, err := n.Conn1.ExecNeo(tmpExportQuery, nil)
	if err != nil {
		return err
	}

	// Replace run ID part of node ID in saved queries.
	sedIDLong := fmt.Sprintf("s/`id`:\"run_0/`id`:\"run_%d/g", newID)
	cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDLong, "/tmp/clean-prov")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed diffprov run ID command: %s", out)
	}

	// Replace run ID in saved queries.
	sedIDShort := fmt.Sprintf("s/`run`:0/`run`:%d/g", newID)
	cmd = exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDShort, "/tmp/clean-prov")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed diffprov run ID command: %s", out)
	}

	// Import modified difference graph as new one.
	_, err = n.Conn1.ExecNeo(`CALL apoc.cypher.runFile("/tmp/clean-prov", {statistics: false});`, nil)
	if err != nil {
		return err
	}

	return nil
}

// PreprocessProv
func (n *Neo4J) PreprocessProv(iters []uint) error {

	fmt.Printf("Preprocessing provenance graphs... ")

	for i := range iters {

		// Clean-copy precondition provenance (run: 1000).
		err := n.cleanCopyProv(iters[i], "pre")
		if err != nil {
			return err
		}

		// Clean-copy postcondition provenance (run: 1000).
		err = n.cleanCopyProv(iters[i], "post")
		if err != nil {
			return err
		}

		// Do preprocessing over run: 1000 graphs:

		// Collapse @next chains
		// MATCH path = (r1:Rule {run: 1000, condition: "post", type: "next"})-[*1]->(g:Goal {run: 1000, condition: "post"})-[*1]->(r2:Rule {run: 1000, condition: "post", type: "next"})
		// RETURN path;

		// What more?

	}

	fmt.Printf("done\n\n")

	return nil
}
