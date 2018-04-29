package graphing

import (
	"fmt"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Functions.

// createDOT
func createDOT(edges []graph.Path) (string, error) {

	dotGraph := gographviz.NewGraph()

	// Name the DOT graph.
	err := dotGraph.SetName("dataflow")
	if err != nil {
		return "", err
	}

	// It is a directed graph.
	err = dotGraph.SetDir(true)
	if err != nil {
		return "", err
	}

	for i := range edges {

		from := edges[i].Nodes[0].Properties["id"].(string)
		to := edges[i].Nodes[1].Properties["id"].(string)

		fromAttrs := make(map[string]string)
		fromAttrs["label"] = fmt.Sprintf("\"%s\"", edges[i].Nodes[0].Properties["label"])

		toAttrs := make(map[string]string)
		toAttrs["label"] = fmt.Sprintf("\"%s\"", edges[i].Nodes[1].Properties["label"])

		// Add first node with all info from query.
		err := dotGraph.AddNode("dataflow", from, fromAttrs)
		if err != nil {
			return "", err
		}

		// Add second node with all info from query.
		err = dotGraph.AddNode("dataflow", to, toAttrs)
		if err != nil {
			return "", err
		}

		// Add edge to DOT graph.
		err = dotGraph.AddEdge(from, to, true, map[string]string{
			"color": "\"black\"",
		})
		if err != nil {
			return "", err
		}
	}

	return dotGraph.String(), nil
}
