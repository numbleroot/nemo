package graphing

import (
	"fmt"
	// "strings"

	// "os/exec"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// extractIntersectionPrototype
func (n *Neo4J) extractIntersectionPrototype(iters []uint, condition string) error {

	// Find all labels of next chain rules.
	stmtCondRules, err := n.Conn1.PrepareNeo(`
	MATCH path = (root:Goal {run: {run}, condition: {condition}})-[*1]->(r1:Rule {run: {run}, condition: {condition}})-[*1..]->(r2:Rule {run: {run}, condition: {condition}})
	OPTIONAL MATCH (g:Goal {run: {run}, condition: "pre", condition_holds: true})
	WITH nodes(path) AS nodes, root, collect(g) AS existsSuccess, length(path) AS len
	WHERE size(existsSuccess) > 0 AND not(()-->(root))
	WITH filter(node IN nodes WHERE exists(node.type)) AS rules, len
	UNWIND rules AS rule
	WITH collect(rule) AS rules, len
	ORDER BY len DESC
	LIMIT 1
	RETURN rules;
    `)
	if err != nil {
		return err
	}

	var protoProv string
	achvdCond := 0
	iterProv := make([][]graph.Node, len(iters))
	numPresent := make([]map[string]int, len(iters))

	for i := range iters {

		numPresent[i] = make(map[string]int)

		// Request all rule labels as long as the
		// execution eventually achieved its condition.
		condRules, err := stmtCondRules.QueryNeo(map[string]interface{}{
			"run":       (1000 + iters[i]),
			"condition": condition,
		})
		if err != nil {
			return err
		}

		condAllRules, _, err := condRules.All()
		if err != nil {
			return err
		}

		err = condRules.Close()
		if err != nil {
			return err
		}

		for j := range condAllRules {

			for k := range condAllRules[j] {

				rulesRaw := condAllRules[j][k].([]interface{})
				rules := make([]graph.Node, len(rulesRaw))

				for l := range rules {

					rules[l] = rulesRaw[l].(graph.Node)

					// Count the number of times a label is present
					// in this particular provenance graph.
					label := rules[l].Properties["label"].(string)
					numPresent[i][label] += 1
				}

				if len(rules) > 0 {

					// Count how many times the precondition was achieved.
					achvdCond += 1

					// Add rules slice to tracking structure.
					iterProv[i] = rules
				}
			}
		}
	}

	for i := range iterProv[0] {

		foundIn := 1
		label0 := iterProv[0][i].Properties["label"].(string)

		for j := 1; j < len(iterProv); j++ {

			if len(iterProv[j]) > 0 {

				for k := range iterProv[j] {

					labelJ := iterProv[j][k].Properties["label"].(string)

					if (label0 == labelJ) && (numPresent[j][label0] > 0) {

						// Mark label as part of the intersection.
						foundIn++

						// Reduce number of times this label is present.
						numPresent[0][label0] -= 1
						numPresent[j][labelJ] -= 1
					}
				}
			}
		}

		if foundIn == achvdCond {

			if protoProv == "" {
				protoProv = fmt.Sprintf("%s", label0)
			} else {
				protoProv = fmt.Sprintf("%s ---> %s", protoProv, label0)
			}
		}
	}

	fmt.Printf("EXTRACTED LABELS OF RULES FOR CONDITION '%s':\n'%v'\n\n", condition, protoProv)

	err = stmtCondRules.Close()
	if err != nil {
		return err
	}

	return nil
}

// extractUnionPrototype
func (n *Neo4J) extractUnionPrototype(iters []uint, condition string) error {
	return nil
}

// CreatePrototype
func (n *Neo4J) CreatePrototype(iters []uint) (*gographviz.Graph, *gographviz.Graph, error) {

	fmt.Printf("Running extraction of success prototype... ")

	// Create precondition prototype.
	err := n.extractIntersectionPrototype(iters, "pre")
	if err != nil {
		return nil, nil, err
	}

	// Create postcondition prototype.
	err = n.extractIntersectionPrototype(iters, "post")
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
