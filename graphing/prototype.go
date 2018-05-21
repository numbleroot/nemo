package graphing

import (
	"fmt"
)

// CreatePrototype
func (n *Neo4J) CreatePrototype(iters []uint) error {

	fmt.Printf("Running extraction of success prototype...")

	stmtCondGoals, err := n.Conn1.PrepareNeo(`
        MATCH (g1:Goal {run: {run}, condition: {condition}})
        OPTIONAL MATCH (g2:Goal {run: {run}, condition: {condition}, condition_holds: true})
        WITH g1, collect(g2) AS existsSuccess
        WHERE size(existsSuccess) > 0
        RETURN collect(g1.label) AS goals;
    `)
	if err != nil {
		return err
	}

	for i := range iters {

		// Request all goal labels as long as the
		// execution eventually achieved its condition.
		condGoals, err := stmtCondGoals.QueryNeo(map[string]interface{}{
			"run":       iters[i],
			"condition": "post",
		})
		if err != nil {
			return err
		}

		condGoalsAll, _, err := condGoals.All()
		if err != nil {
			return err
		}

		err = condGoals.Close()
		if err != nil {
			return err
		}

		fmt.Printf("\n%d:\n'%#v'\n", i, condGoalsAll)
	}

	err = stmtCondGoals.Close()
	if err != nil {
		return err
	}

	fmt.Printf(" done\n\n")

	return nil
}
