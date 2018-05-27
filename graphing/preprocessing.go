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
	YIELD time
	RETURN time;`

	exportQuery = strings.Replace(exportQuery, "###RUN###", fmt.Sprintf("%d", iter), -1)
	exportQuery = strings.Replace(exportQuery, "###CONDITION###", condition, -1)
	_, err := n.Conn1.ExecNeo(exportQuery, nil)
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

	run := (1000 + iter)

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
		"run":       run,
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
	nextChainIDs := make([][]int64, 0, len(nextPathsAll))

	// Create map to quickly check node containment in path.
	nextChainsNodes := make(map[int64]bool)

	for j := range nextPathsAll {

		newChain := false
		paths := nextPathsAll[j][0].(graph.Path)

		nodesRaw := nextPathsAll[j][1].([]interface{})
		nodes := make([]int64, len(nodesRaw))

		for n := range nodes {

			nodes[n] = nodesRaw[n].(int64)

			_, found := nextChainsNodes[nodes[n]]
			if !found {
				newChain = true
			}
		}

		if newChain {

			// Add these next chain paths to global structure.
			nextChains = append(nextChains, paths.Nodes)
			nextChainIDs = append(nextChainIDs, nodes)

			// Also add contained node labels to map so that
			// we can decide on future paths.
			for n := range nodes {
				nextChainsNodes[nodes[n]] = true
			}
		}
	}

	err = stmtCollapseNext.Close()
	if err != nil {
		return err
	}

	// Find predecessor relations to chain.
	stmtPred, err := n.Conn1.PrepareNeo(`
	MATCH (pred:Goal {run: {run}, condition: {condition}})-[*1]->(root:Rule {run: {run}, condition: {condition}})
	WHERE ID(root) = {rootID}
	WITH collect(ID(pred)) AS preds
	RETURN preds;
	`)
	if err != nil {
		return err
	}

	preds := make([][]int64, len(nextChains))

	for i := range nextChains {

		predsRaw, err := stmtPred.QueryNeo(map[string]interface{}{
			"run":       run,
			"condition": condition,
			"rootID":    nextChainIDs[i][0],
		})
		if err != nil {
			return err
		}

		predsAll, _, err := predsRaw.All()
		if err != nil {
			return err
		}

		err = predsRaw.Close()
		if err != nil {
			return err
		}

		preds[i] = make([]int64, 0, 1)

		for p := range predsAll {

			// Extract all predecessor nodes and append them
			// individually to the global tracking structure.
			predsParsed := predsAll[p][0].([]interface{})
			for r := range predsParsed {
				preds[i] = append(preds[i], predsParsed[r].(int64))
			}
		}
	}

	err = stmtPred.Close()
	if err != nil {
		return err
	}

	// Find all "outwards" relations of chain.
	stmtSucc, err := n.Conn2.PrepareNeo(`
	MATCH (leaf:Rule {run: {run}, condition: {condition}})-[*1]->(succ:Goal {run: {run}, condition: {condition}})
	WHERE ID(leaf) = {leafID}
	WITH collect(ID(succ)) AS succs
	RETURN succs;
	`)
	if err != nil {
		return err
	}

	succs := make([][]int64, len(nextChains))

	for i := range nextChains {

		succsRaw, err := stmtSucc.QueryNeo(map[string]interface{}{
			"run":       run,
			"condition": condition,
			"leafID":    nextChainIDs[i][(len(nextChainIDs[i]) - 1)],
		})
		if err != nil {
			return err
		}

		succsAll, _, err := succsRaw.All()
		if err != nil {
			return err
		}

		err = succsRaw.Close()
		if err != nil {
			return err
		}

		succs[i] = make([]int64, 0, 1)

		for p := range succsAll {

			// Extract all successor nodes and append them
			// individually to the global tracking structure.
			succsParsed := succsAll[p][0].([]interface{})
			for r := range succsParsed {
				succs[i] = append(succs[i], succsParsed[r].(int64))
			}
		}
	}

	err = stmtSucc.Close()
	if err != nil {
		return err
	}

	for i := range nextChains {

		label := fmt.Sprintf("%s_collapsed", nextChains[i][0].Properties["table"])
		id := fmt.Sprintf("run_%d_%s_%s", run, condition, label)

		var predsIDs string
		for j := range preds[i] {

			if predsIDs == "" {
				predsIDs = fmt.Sprintf("[%d", preds[i][j])
			} else {
				predsIDs = fmt.Sprintf("%s, %d", predsIDs, preds[i][j])
			}
		}
		predsIDs = fmt.Sprintf("%s]", predsIDs)

		var succsIDs string
		for j := range succs[i] {

			if succsIDs == "" {
				succsIDs = fmt.Sprintf("[%d", succs[i][j])
			} else {
				succsIDs = fmt.Sprintf("%s, %d", succsIDs, succs[i][j])
			}
		}
		succsIDs = fmt.Sprintf("%s]", succsIDs)

		// Create new nodes representing the intent of the
		// captured @next chains.
		_, err := n.Conn1.ExecNeo(`
		CREATE (repl:Rule {run: {run}, condition: {condition}, id: {id}, label: {label}, table: {table}, type: 'collapsed'});
		`, map[string]interface{}{
			"run":       run,
			"condition": condition,
			"id":        id,
			"label":     label,
			"table":     nextChains[i][0].Properties["table"],
		})
		if err != nil {
			return err
		}

		// Connect newly created collapsed next node with
		// predecessors and successors.
		addPredsSuccsQuery := `
		MATCH (pred:Goal {run: ###RUN###, condition: "###CONDITION###"}), (coll:Rule {run: ###RUN###, condition: "###CONDITION###", id: "###ID###", type: "collapsed"}), (succ:Goal {run: ###RUN###, condition: "###CONDITION###"})
		WHERE ID(pred) IN ###PRED_IDs### AND ID(succ) IN ###SUCC_IDs###
		MERGE (pred)-[:DUETO]->(coll)
		MERGE (coll)-[:DUETO]->(succ);
		`
		addPredsSuccsQuery = strings.Replace(addPredsSuccsQuery, "###RUN###", fmt.Sprintf("%d", run), -1)
		addPredsSuccsQuery = strings.Replace(addPredsSuccsQuery, "###CONDITION###", condition, -1)
		addPredsSuccsQuery = strings.Replace(addPredsSuccsQuery, "###ID###", id, -1)
		addPredsSuccsQuery = strings.Replace(addPredsSuccsQuery, "###PRED_IDs###", predsIDs, -1)
		addPredsSuccsQuery = strings.Replace(addPredsSuccsQuery, "###SUCC_IDs###", succsIDs, -1)

		_, err = n.Conn2.ExecNeo(addPredsSuccsQuery, nil)
		if err != nil {
			return err
		}
	}

	// Delete extracted next chain.
	stmtDelChainRaw := `
	MATCH path = (r:Rule {run: {run}, condition: {condition}, type: "next"})-[*1..]->(g:Goal {run: {run}, condition: {condition}})-[*1..]->(l:Rule {run: {run}, condition: {condition}, type: "next"})
	WHERE all(node IN nodes(path) WHERE ID(node) IN ###CHAIN_IDs###)
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
	stmtDelChainRaw = strings.Replace(stmtDelChainRaw, "###CHAIN_IDs###", fmt.Sprintf("[%s]", deleteIDsString), -1)

	stmtDelChain, err := n.Conn1.PrepareNeo(stmtDelChainRaw)
	if err != nil {
		return err
	}

	_, err = stmtDelChain.ExecNeo(map[string]interface{}{
		"run":       run,
		"condition": condition,
	})
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

		// Collapse @next chains in precondition provenance.
		err = n.collapseNextChains(iters[i], "pre")
		if err != nil {
			return err
		}

		// Collapse @next chains in postcondition provenance.
		err = n.collapseNextChains(iters[i], "post")
		if err != nil {
			return err
		}

		// What more?

	}

	fmt.Printf("done\n\n")

	return nil
}
