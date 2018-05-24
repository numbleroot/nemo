package graphing

import (
	"fmt"
)

// PreprocessProv
func (n *Neo4J) PreprocessProv(iters []uint) error {

	fmt.Printf("Preprocessing provenance graphs... ")

	// Range over all iters.

	// Clean-copy precondition provenance (run: 1000).

	// Clean-copy postcondition provenance (run: 1000).

	// Do preprocessing over run: 1000 graphs:

	// Collapse @next chains
	/*
		MATCH path = (r1:Rule {run: 1000, condition: "post", type: "next"})-[*1]->(g:Goal {run: 1000, condition: "post"})-[*1]->(r2:Rule {run: 1000, condition: "post", type: "next"})
		RETURN path;
	*/

	// What more?

	fmt.Printf("done\n\n")

	return nil
}
