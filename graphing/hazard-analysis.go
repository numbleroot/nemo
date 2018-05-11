package graphing

import (
	"fmt"
	"strings"

	"io/ioutil"
	"path/filepath"

	"github.com/awalterschulze/gographviz"
)

// Functions.

// CreateHazardAnalysis
func (n *Neo4J) CreateHazardAnalysis(faultInjOut string) ([]*gographviz.Graph, error) {

	fmt.Printf("Running hazard window analysis...")

	dots := make([]*gographviz.Graph, len(n.Runs))

	for i := range n.Runs {

		// Space-time file name in fault injector directory.
		fiSpaceTime := filepath.Join(faultInjOut, fmt.Sprintf("run_%d_spacetime.dot", n.Runs[i].Iteration))

		// Load current space-time diagram.
		spaceTimeDotBytes, err := ioutil.ReadFile(fiSpaceTime)
		if err != nil {
			return nil, err
		}

		// Read DOT data.
		spaceTimeGraph, err := gographviz.Read(spaceTimeDotBytes)
		if err != nil {
			return nil, err
		}

		for j := range spaceTimeGraph.Nodes.Nodes {

			spaceTimeGraph.Nodes.Nodes[j].Attrs.Extend(map[gographviz.Attr]string{
				"style":     "\"solid, filled\"",
				"color":     "\"lightgrey\"",
				"fillcolor": "\"lightgrey\"",
			})

			// Split into naming and time parts.
			nameParts := strings.Split(spaceTimeGraph.Nodes.Nodes[j].Name, "_")

			// Possibly selecting the time of the node here.
			// If this is not actually the time, it does not
			// pose a problem as our map below only works on
			// actual timesteps.
			nodeTime := nameParts[(len(nameParts) - 1)]

			// TODO: If Pre() is not specified over only
			//       one global column, but rather over
			//       node-individual local state, we have
			//       to proceed differently here!
			_, preHolds := n.Runs[i].TimePreHolds[nodeTime]
			if preHolds {

				spaceTimeGraph.Nodes.Nodes[j].Attrs.Extend(map[gographviz.Attr]string{
					"color":     "\"firebrick\"",
					"fillcolor": "\"firebrick\"",
				})
			}

			// TODO: If Post() is not specified over only
			//       one global column, but rather over
			//       node-individual local state, we have
			//       to proceed differently here!
			_, postHolds := n.Runs[i].TimePostHolds[nodeTime]
			if postHolds {

				spaceTimeGraph.Nodes.Nodes[j].Attrs.Extend(map[gographviz.Attr]string{
					"fillcolor": "\"deepskyblue\"",
				})
			}
		}

		dots[i] = spaceTimeGraph
	}

	fmt.Printf(" done\n\n")

	return dots, nil
}
