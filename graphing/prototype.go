package graphing

import (
	"fmt"
)

// extractProtos extracts the intersection-prototype
// and union-prototype from all iterations.
func (n *Neo4J) extractProtos(iters []uint, condition string) ([]string, []string, error) {

	stmtCondRules, err := n.Conn1.PrepareNeo(`
		MATCH path = (root:Goal {run: {run}, condition: {condition}})-[*1]->(r1:Rule {run: {run}, condition: {condition}})-[*1..]->(r2:Rule {run: {run}, condition: {condition}})
		OPTIONAL MATCH (g:Goal {run: {run}, condition: "pre", condition_holds: true})
		WITH path, root, collect(g) AS existsSuccess, length(path) AS len
		WHERE size(existsSuccess) > 0 AND not(()-->(root))
		WITH path, len
		ORDER BY len DESC
		WITH collect(nodes(path)) AS nodes
		WITH reduce(output = [], node IN nodes | output + node) AS nodes
		WITH filter(node IN nodes WHERE exists(node.type)) AS rules
		UNWIND rules AS rule
		WITH collect(DISTINCT rule.label) AS rules
		RETURN rules;
    `)
	if err != nil {
		return nil, nil, err
	}

	achvdCond := 0
	interProto := make([]string, 0, 10)
	unionProto := make([]string, 0, 10)
	iterProv := make([][]string, len(iters))

	for i := range iters {

		// Request all rule labels as long as the
		// execution eventually achieved its condition.
		condRules, err := stmtCondRules.QueryNeo(map[string]interface{}{
			"run":       (1000 + iters[i]),
			"condition": condition,
		})
		if err != nil {
			return nil, nil, err
		}

		condAllRules, _, err := condRules.All()
		if err != nil {
			return nil, nil, err
		}

		err = condRules.Close()
		if err != nil {
			return nil, nil, err
		}

		for j := range condAllRules {

			for k := range condAllRules[j] {

				rulesRaw := condAllRules[j][k].([]interface{})
				rules := make([]string, len(rulesRaw))

				for l := range rules {
					rules[l] = rulesRaw[l].(string)
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

	// Initially, set first chain as longest.
	longest := len(iterProv[0])

	for i := range iterProv[0] {

		foundIn := 1

		for j := 1; j < len(iterProv); j++ {

			if len(iterProv[j]) > 0 {

				for k := range iterProv[j] {

					// If found, mark label as part of the intersection.
					if iterProv[0][i] == iterProv[j][k] {
						foundIn++
					}
				}
			}

			// Update longest if necessary.
			if len(iterProv[j]) > longest {
				longest = len(iterProv[j])
			}
		}

		// If in intersection, append label to final prototype.
		if (foundIn == achvdCond) && (iterProv[0][i] != condition) {
			interProto = append(interProto, iterProv[0][i])
		}
	}

	// Keep track of rules we already saw.
	alreadySeen := make(map[string]bool)

	for i := 0; i < longest; i++ {

		for j := range iterProv {

			if i < len(iterProv[j]) {

				if !alreadySeen[iterProv[j][i]] && (iterProv[j][i] != condition) {

					// New label, add to union.
					unionProto = append(unionProto, iterProv[j][i])

					// Update map to seen for this label.
					alreadySeen[iterProv[j][i]] = true
				}
			}
		}
	}

	err = stmtCondRules.Close()
	if err != nil {
		return nil, nil, err
	}

	return interProto, unionProto, nil
}

// missingFrom
func (n *Neo4J) missingFrom(proto []string, failedIter uint, condition string) ([]string, error) {

	stmtMissRules, err := n.Conn1.PrepareNeo(`
		MATCH (r:Rule {run: {run}, condition: {condition}})
		WITH collect(DISTINCT r.label) AS rules
		RETURN rules;
    `)
	if err != nil {
		return nil, err
	}

	missRules, err := stmtMissRules.QueryNeo(map[string]interface{}{
		"run":       (1000 + failedIter),
		"condition": condition,
	})
	if err != nil {
		return nil, err
	}

	missAllRules, _, err := missRules.All()
	if err != nil {
		return nil, err
	}

	err = missRules.Close()
	if err != nil {
		return nil, err
	}

	failedRules := make(map[string]bool)

	for j := range missAllRules {

		for k := range missAllRules[j] {

			rulesRaw := missAllRules[j][k].([]interface{})
			rules := make([]string, len(rulesRaw))

			for l := range rules {

				rules[l] = rulesRaw[l].(string)

				// Add rule to tracking structure.
				failedRules[rules[l]] = true
			}
		}
	}

	// Figure out the difference in rules
	// between prototype and failed run's rules.
	missing := make([]string, 0, 3)

	for p := range proto {

		if !failedRules[proto[p]] {
			missing = append(missing, fmt.Sprintf("<code>%s</code>", proto[p]))
		}
	}

	err = stmtMissRules.Close()
	if err != nil {
		return nil, err
	}

	return missing, nil
}

// CreatePrototypes
func (n *Neo4J) CreatePrototypes(iters []uint, failedIters []uint) ([]string, [][]string, []string, [][]string, error) {

	fmt.Printf("Running extraction of success prototypes... ")

	// In the future, we might want to add
	// analysis of precondition prototypes.

	// Create postcondition intersection-prototype
	// and union-prototype.
	interProto, unionProto, err := n.extractProtos(iters, "post")
	if err != nil {
		return nil, nil, nil, nil, err
	}

	interProtoMiss := make([][]string, len(failedIters))
	unionProtoMiss := make([][]string, len(failedIters))

	for i := range failedIters {

		// Collect all nodes missing in the failed execution's postcondition
		// provenance that are part of the intersection-prototype.
		interMiss, err := n.missingFrom(interProto, failedIters[i], "post")
		if err != nil {
			return nil, nil, nil, nil, err
		}
		interProtoMiss[i] = interMiss

		// Collect all nodes missing in the failed execution's postcondition
		// provenance that are part of the union-prototype.
		unionMiss, err := n.missingFrom(unionProto, failedIters[i], "post")
		if err != nil {
			return nil, nil, nil, nil, err
		}
		unionProtoMiss[i] = unionMiss
	}

	for i := range interProto {
		interProto[i] = fmt.Sprintf("<code>%s</code>", interProto[i])
	}

	for i := range unionProto {
		unionProto[i] = fmt.Sprintf("<code>%s</code>", unionProto[i])
	}

	fmt.Printf("done\n\n")

	return interProto, interProtoMiss, unionProto, unionProtoMiss, nil
}
