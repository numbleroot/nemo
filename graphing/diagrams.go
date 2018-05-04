package graphing

import (
	"fmt"
	"strings"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Functions.

// createDOT
func createDOT(edges []graph.Path, graphType string) (*gographviz.Graph, error) {

	dotGraph := gographviz.NewGraph()

	// Name the DOT graph.
	err := dotGraph.SetName("dataflow")
	if err != nil {
		return nil, err
	}

	// It is a directed graph.
	err = dotGraph.SetDir(true)
	if err != nil {
		return nil, err
	}

	for i := range edges {

		from := edges[i].Nodes[0].Properties["id"].(string)
		to := edges[i].Nodes[1].Properties["id"].(string)

		fromAttrs := make(map[string]string)

		fromAttrs["label"] = fmt.Sprintf("\"%s\"", edges[i].Nodes[0].Properties["label"])
		fromAttrs["style"] = "\"filled, solid\""
		fromAttrs["color"] = "\"black\""
		fromAttrs["fontcolor"] = "\"black\""
		fromAttrs["fillcolor"] = "\"white\""

		// Style node differently based on time notion.
		if edges[i].Nodes[0].Properties["type"] == "async" {
			fromAttrs["style"] = "\"filled, bold\""
			fromAttrs["color"] = "\"orangered\""
		}

		// Style node differently based on achieved condition.
		if (edges[i].Nodes[0].Properties["condition_holds"] == true) && (graphType == "pre") {
			fromAttrs["color"] = "\"firebrick3\""
			fromAttrs["fillcolor"] = "\"firebrick3\""
		} else if (edges[i].Nodes[0].Properties["condition_holds"] == true) && (graphType == "post") {
			fromAttrs["color"] = "\"deepskyblue3\""
			fromAttrs["fillcolor"] = "\"deepskyblue3\""
		}

		toAttrs := make(map[string]string)

		toAttrs["label"] = fmt.Sprintf("\"%s\"", edges[i].Nodes[1].Properties["label"])
		toAttrs["style"] = "\"filled, solid\""
		toAttrs["color"] = "\"black\""
		toAttrs["fontcolor"] = "\"black\""
		toAttrs["fillcolor"] = "\"white\""

		// Style node differently based on time notion.
		if edges[i].Nodes[1].Properties["type"] == "async" {
			toAttrs["style"] = "\"filled, bold\""
			toAttrs["color"] = "\"orangered\""
		}

		// Style node differently based on achieved condition.
		if (edges[i].Nodes[1].Properties["condition_holds"] == true) && (graphType == "pre") {
			toAttrs["color"] = "\"firebrick3\""
			toAttrs["fillcolor"] = "\"firebrick3\""
		} else if (edges[i].Nodes[1].Properties["condition_holds"] == true) && (graphType == "post") {
			toAttrs["color"] = "\"deepskyblue3\""
			toAttrs["fillcolor"] = "\"deepskyblue3\""
		}

		// Add first node with all info from query.
		err := dotGraph.AddNode("dataflow", from, fromAttrs)
		if err != nil {
			return nil, err
		}

		// Add second node with all info from query.
		err = dotGraph.AddNode("dataflow", to, toAttrs)
		if err != nil {
			return nil, err
		}

		// Add edge to DOT graph.
		err = dotGraph.AddEdge(from, to, true, map[string]string{
			"color": "\"black\"",
		})
		if err != nil {
			return nil, err
		}
	}

	return dotGraph, nil
}

// createDiffDot
func createDiffDot(diffRunID uint, edges []graph.Path, successRunID uint, successPostProv *gographviz.Graph) (*gographviz.Graph, error) {

	dotGraph := gographviz.NewGraph()

	// Name the DOT graph.
	err := dotGraph.SetName("dataflow")
	if err != nil {
		return nil, err
	}

	// It is a directed graph.
	err = dotGraph.SetDir(true)
	if err != nil {
		return nil, err
	}

	for _, edge := range successPostProv.Edges.Edges {

		diffSrc := strings.Replace(edge.Src, fmt.Sprintf("run_%d", successRunID), fmt.Sprintf("run_%d", diffRunID), -1)
		diffDst := strings.Replace(edge.Dst, fmt.Sprintf("run_%d", successRunID), fmt.Sprintf("run_%d", diffRunID), -1)

		// Copy attribute map.
		attrMap := make(map[string]string)
		for j := range edge.Attrs {
			attrMap[string(j)] = edge.Attrs[j]
		}

		// Copy the edge over to new graph.
		err := dotGraph.AddEdge(diffSrc, diffDst, edge.Dir, attrMap)
		if err != nil {
			return nil, err
		}
	}

	// Make all edges invisible before copying
	// successful postcondition provenance graph.
	err = dotGraph.AddNode("dataflow", "edge", map[string]string{
		"style": "\"invis\"",
	})
	if err != nil {
		return nil, err
	}

	for _, node := range successPostProv.Nodes.Nodes {

		diffName := strings.Replace(node.Name, fmt.Sprintf("run_%d", successRunID), fmt.Sprintf("run_%d", diffRunID), -1)

		// Copy attribute map.
		attrMap := make(map[string]string)
		for j := range node.Attrs {
			attrMap[string(j)] = node.Attrs[j]
		}

		// Overwrite style attribute to hide node.
		attrMap["style"] = "\"invis\""

		// Copy the node over to new graph.
		err := dotGraph.AddNode("dataflow", diffName, attrMap)
		if err != nil {
			return nil, err
		}
	}

	for i := range edges {

		from := edges[i].Nodes[0].Properties["id"].(string)
		to := edges[i].Nodes[1].Properties["id"].(string)

		dotGraph.Nodes.Lookup[from].Attrs["style"] = "\"filled, solid\""
		dotGraph.Nodes.Lookup[to].Attrs["style"] = "\"filled, solid\""

		for j := range dotGraph.Edges.SrcToDsts[from][to] {
			fmt.Printf("EDGE: '%#v'\n", dotGraph.Edges.SrcToDsts[from][to][j])
		}
	}

	return dotGraph, err
}
