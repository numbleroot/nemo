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

		fmt.Printf("FROM: '%#v'\n\n", edges[i].Nodes[0])
		fmt.Printf("TO: '%#v'\n\n", edges[i].Nodes[1])

		from := edges[i].Nodes[0].Properties["id"].(string)
		to := edges[i].Nodes[1].Properties["id"].(string)

		fromAttrs := make(map[string]string)
		fromAttrs["label"] = fmt.Sprintf("\"%s\"", edges[i].Nodes[0].Properties["label"])

		// Style node differently based on time notion.
		if edges[i].Nodes[0].Properties["type"] == "async" {
			fromAttrs["style"] = "\"filled, bold\""
			fromAttrs["color"] = "\"black\""
			fromAttrs["fontcolor"] = "\"black\""
			fromAttrs["fillcolor"] = "\"steelblue1\""
		} else if edges[i].Nodes[0].Properties["type"] == "next" {
			fromAttrs["style"] = "\"filled, solid\""
			fromAttrs["color"] = "\"gray50\""
			fromAttrs["fontcolor"] = "\"gray50\""
			fromAttrs["fillcolor"] = "\"limegreen\""
		} else {
			fromAttrs["style"] = "\"filled, solid\""
			fromAttrs["color"] = "\"gray50\""
			fromAttrs["fontcolor"] = "\"gray50\""
			fromAttrs["fillcolor"] = "\"white\""
		}

		// Style node differently based on achieved condition.
		if edges[i].Nodes[0].Properties["condition_holds"] == true {
			fromAttrs["fontcolor"] = "\"crimson\""
		}

		toAttrs := make(map[string]string)
		toAttrs["label"] = fmt.Sprintf("\"%s\"", edges[i].Nodes[1].Properties["label"])

		// Style node differently based on time notion.
		if edges[i].Nodes[1].Properties["type"] == "async" {
			toAttrs["style"] = "\"filled, bold\""
			toAttrs["color"] = "\"black\""
			toAttrs["fontcolor"] = "\"black\""
			toAttrs["fillcolor"] = "\"steelblue1\""
		} else if edges[i].Nodes[1].Properties["type"] == "next" {
			toAttrs["style"] = "\"filled, solid\""
			toAttrs["color"] = "\"gray50\""
			toAttrs["fontcolor"] = "\"gray50\""
			toAttrs["fillcolor"] = "\"limegreen\""
		} else {
			toAttrs["style"] = "\"filled, solid\""
			toAttrs["color"] = "\"gray50\""
			toAttrs["fontcolor"] = "\"gray50\""
			toAttrs["fillcolor"] = "\"white\""
		}

		// Style node differently based on achieved condition.
		if edges[i].Nodes[1].Properties["condition_holds"] == true {
			toAttrs["fontcolor"] = "\"crimson\""
		}

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
