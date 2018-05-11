package graphing

import (
	"fmt"
	"strings"

	"github.com/awalterschulze/gographviz"
	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	fi "github.com/numbleroot/nemo/faultinjectors"
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

	// Make sure the background is transparent.
	err = dotGraph.AddNode("dataflow", "graph", map[string]string{
		"bgcolor": "\"transparent\"",
	})
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
			fromAttrs["color"] = "\"springgreen3\""
		}

		// Style node differently based on achieved condition.
		if (edges[i].Nodes[0].Properties["condition_holds"] == true) && (graphType == "pre") {
			fromAttrs["color"] = "\"firebrick3\""
			fromAttrs["fillcolor"] = "\"firebrick3\""
		} else if (edges[i].Nodes[0].Properties["condition_holds"] == true) && (graphType == "post") {
			fromAttrs["color"] = "\"deepskyblue3\""
			fromAttrs["fillcolor"] = "\"deepskyblue3\""
		}

		// Alter shape based on being rule or goal.
		if edges[i].Nodes[0].Labels[0] == "Rule" {
			fromAttrs["shape"] = "rect"
		} else {
			fromAttrs["shape"] = "ellipse"
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
			toAttrs["color"] = "\"springgreen3\""
		}

		// Style node differently based on achieved condition.
		if (edges[i].Nodes[1].Properties["condition_holds"] == true) && (graphType == "pre") {
			toAttrs["color"] = "\"firebrick3\""
			toAttrs["fillcolor"] = "\"firebrick3\""
		} else if (edges[i].Nodes[1].Properties["condition_holds"] == true) && (graphType == "post") {
			toAttrs["color"] = "\"deepskyblue3\""
			toAttrs["fillcolor"] = "\"deepskyblue3\""
		}

		// Alter shape based on being rule or goal.
		if edges[i].Nodes[1].Labels[0] == "Rule" {
			toAttrs["shape"] = "rect"
		} else {
			toAttrs["shape"] = "ellipse"
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
func createDiffDot(diffRunID uint, diffEdges []graph.Path, failedRunID uint, failedEdges []graph.Path, successRunID uint, successPostProv *gographviz.Graph, missing *fi.Missing) (*gographviz.Graph, *gographviz.Graph, error) {

	// Create map for lookup of missing events.
	missingMap := make(map[string]bool)
	missingMap[missing.Rule.ID] = true
	for m := range missing.Goals {
		missingMap[missing.Goals[m].ID] = true
	}

	diffDotGraph := gographviz.NewGraph()
	failedDotGraph := gographviz.NewGraph()

	// Name the DOT graphs.
	err := diffDotGraph.SetName("dataflow")
	if err != nil {
		return nil, nil, err
	}

	err = failedDotGraph.SetName("dataflow")
	if err != nil {
		return nil, nil, err
	}

	// They both are directed graph.
	err = diffDotGraph.SetDir(true)
	if err != nil {
		return nil, nil, err
	}

	err = failedDotGraph.SetDir(true)
	if err != nil {
		return nil, nil, err
	}

	// Make sure both backgrounds are transparent.
	err = diffDotGraph.AddNode("dataflow", "graph", map[string]string{
		"bgcolor": "\"transparent\"",
	})
	if err != nil {
		return nil, nil, err
	}

	err = failedDotGraph.AddNode("dataflow", "graph", map[string]string{
		"bgcolor": "\"transparent\"",
	})
	if err != nil {
		return nil, nil, err
	}

	for _, edge := range successPostProv.Edges.Edges {

		diffSrc := strings.Replace(edge.Src, fmt.Sprintf("run_%d", successRunID), fmt.Sprintf("run_%d", diffRunID), -1)
		diffDst := strings.Replace(edge.Dst, fmt.Sprintf("run_%d", successRunID), fmt.Sprintf("run_%d", diffRunID), -1)

		// Copy attribute map.
		attrMap := make(map[string]string)
		for j := range edge.Attrs {
			attrMap[string(j)] = edge.Attrs[j]
		}

		// Overwrite style attribute to hide edge.
		attrMap["style"] = "\"invis\""

		// Copy the edge over to new graphs.
		err := diffDotGraph.AddEdge(diffSrc, diffDst, edge.Dir, attrMap)
		if err != nil {
			return nil, nil, err
		}

		err = failedDotGraph.AddEdge(diffSrc, diffDst, edge.Dir, attrMap)
		if err != nil {
			return nil, nil, err
		}
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

		// Copy the node over to new graphs.
		err := diffDotGraph.AddNode("dataflow", diffName, attrMap)
		if err != nil {
			return nil, nil, err
		}

		err = failedDotGraph.AddNode("dataflow", diffName, attrMap)
		if err != nil {
			return nil, nil, err
		}
	}

	for i := range diffEdges {

		from := diffEdges[i].Nodes[0].Properties["id"].(string)
		to := diffEdges[i].Nodes[1].Properties["id"].(string)

		// Make nodes visible again that are
		// part of the selected subgraph.
		diffDotGraph.Nodes.Lookup[from].Attrs["style"] = "\"filled, solid\""
		diffDotGraph.Nodes.Lookup[to].Attrs["style"] = "\"filled, solid\""

		// Make edges visible again that are
		// part of the selected subgraph.
		for j := range diffDotGraph.Edges.SrcToDsts[from][to] {
			diffDotGraph.Edges.SrcToDsts[from][to][j].Attrs["style"] = "\"filled, solid\""
		}

		// If one of the nodes is one of the
		// missing events, mark it specifically.
		_, isMissingFrom := missingMap[from]
		if isMissingFrom {
			diffDotGraph.Nodes.Lookup[from].Attrs["style"] = "\"filled, dashed, bold\""
			diffDotGraph.Nodes.Lookup[from].Attrs["color"] = "\"crimson\""
		}

		_, isMissingTo := missingMap[to]
		if isMissingTo {
			diffDotGraph.Nodes.Lookup[to].Attrs["style"] = "\"filled, dashed, bold\""
			diffDotGraph.Nodes.Lookup[to].Attrs["color"] = "\"crimson\""
		}
	}

	for i := range failedEdges {

		from := fmt.Sprintf("\"%s\"", failedEdges[i].Nodes[0].Properties["label"].(string))
		to := fmt.Sprintf("\"%s\"", failedEdges[i].Nodes[1].Properties["label"].(string))

		for j := range failedDotGraph.Nodes.Nodes {

			if (failedDotGraph.Nodes.Nodes[j].Attrs["label"] == from) || (failedDotGraph.Nodes.Nodes[j].Attrs["label"] == to) {
				failedDotGraph.Nodes.Nodes[j].Attrs["style"] = "\"filled, solid\""
			}
		}
	}

	for i := range failedDotGraph.Edges.Edges {

		from := failedDotGraph.Edges.Edges[i].Src
		to := failedDotGraph.Edges.Edges[i].Dst

		if (failedDotGraph.Nodes.Lookup[from].Attrs["style"] == "\"filled, solid\"") && (failedDotGraph.Nodes.Lookup[to].Attrs["style"] == "\"filled, solid\"") {
			failedDotGraph.Edges.Edges[i].Attrs["style"] = "\"filled, solid\""
		}
	}

	return diffDotGraph, failedDotGraph, err
}
