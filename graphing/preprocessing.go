package graphing

import (
	"fmt"
	"strings"

	"os/exec"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
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
	_, err := n.Conn1.ExecNeo(tmpExportQuery, nil)
	if err != nil {
		return err
	}

	// Replace run ID part of node ID in saved queries.
	sedIDLong := fmt.Sprintf("s/`id`:\"run_%d/`id`:\"run_%d/g", iter, newID)
	cmd := exec.Command("sudo", "docker", "exec", "graphdb", "sed", "-i", sedIDLong, "/tmp/clean-prov")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("Wrong return value from docker-compose exec sed diffprov run ID command: %s", out)
	}

	// Replace run ID in saved queries.
	sedIDShort := fmt.Sprintf("s/`run`:%d/`run`:%d/g", iter, newID)
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

// collapseNextChains
func (n *Neo4J) collapseNextChains(iter uint, condition string) error {

	stmtCollapseNext, err := n.Conn2.PrepareNeo(`
	MATCH path = (r1:Rule {run: {run}, condition: {condition}, type: "next"})-[*1..]->(g:Goal {run: {run}, condition: {condition}})-[*1..]->(r2:Rule {run: {run}, condition: {condition}, type: "next"})
	WHERE all(node IN nodes(path) WHERE node.type = "next" OR not(exists(node.type)))
	WITH path, nodes(path) AS nodesRaw, length(path) AS len
	UNWIND nodesRaw AS node
	WITH path, collect(ID(node)) AS nodes, len
	RETURN path, nodes
	ORDER BY len DESC;
	`)
	if err != nil {
		return err
	}

	nextPaths, err := stmtCollapseNext.QueryNeo(map[string]interface{}{
		"run":       (1000 + iter),
		"condition": condition,
	})
	if err != nil {
		return err
	}

	nextPathsAll, _, err := nextPaths.All()
	if err != nil {
		return err
	}

	err = nextPaths.Close()
	if err != nil {
		return err
	}

	// Create structure to track top-level @next chains per iteration.
	nextChains := make([][]graph.Node, 0, len(nextPathsAll))

	// Create map to quickly check node containment in path.
	nextChainsNodes := make(map[int64]bool)

	for j := range nextPathsAll {

		newChain := false
		paths := nextPathsAll[j][0].(graph.Path)
		nodes := nextPathsAll[j][1].([]interface{})

		for n := range nodes {

			_, found := nextChainsNodes[nodes[n].(int64)]
			if !found {
				newChain = true
			}
		}

		if newChain {

			// Add these next chain paths to global structure.
			nextChains = append(nextChains, paths.Nodes)

			// Also add contained node labels to map so that
			// we can decide on future paths.

			for n := range nodes {
				nextChainsNodes[nodes[n].(int64)] = true
			}
		}
	}

	// Find predecessor relations to chain.

	// Find all "outwards" relations of chain.

	// Create new nodes representing the intent of the
	// captured @next chains.
	// Set 'collapsed' = true property artificially for later queries.
	// Connect new node from pred and to all successors (except clock?).

	// Delete extracted next chain.
	stmtDelChainRaw := `
	MATCH path = (r:Rule {run: {run}, condition: {condition}, type: "next"})-[*1..]->(g:Goal {run: {run}, condition: {condition}})-[*1..]->(l:Rule {run: {run}, condition: {condition}, type: "next"})
	WHERE ID(r) IN ###CHAINIDS### AND ID(g) IN ###CHAINIDS### AND ID(l) IN ###CHAINIDS###
	WITH path, nodes(path) AS nodes, length(path) AS len
	ORDER BY len DESC
	UNWIND nodes AS node
	DETACH DELETE node;
	`

	// Create string containing all IDs to delete in Cypher format.
	deleteIDs := make([]string, 0, len(nextChainsNodes))
	for id := range nextChainsNodes {
		deleteIDs = append(deleteIDs, fmt.Sprintf("%d", id))
	}
	deleteIDsString := strings.Join(deleteIDs, ", ")
	stmtDelChainRaw = strings.Replace(stmtDelChainRaw, "###CHAINIDS###", fmt.Sprintf("[%s]", deleteIDsString), -1)

	stmtDelChain, err := n.Conn1.PrepareNeo(stmtDelChainRaw)
	if err != nil {
		return err
	}

	_, err = stmtDelChain.ExecNeo(map[string]interface{}{
		"run":       (1000 + iter),
		"condition": condition,
	})
	if err != nil {
		return err
	}

	err = stmtCollapseNext.Close()
	if err != nil {
		return err
	}

	err = stmtDelChain.Close()
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

		// Collapse @next chains.
		// Find all paths that represent @next chains, ordered by length.
		// Build local map of key: nodeLabel, value: successorInPath.
		// For every path returned, check if contained in map.

		err = n.collapseNextChains(iters[i], "pre")
		if err != nil {
			return err
		}

		err = n.collapseNextChains(iters[i], "post")
		if err != nil {
			return err
		}

		// What more?

	}

	fmt.Printf("done\n\n")

	return nil
}
