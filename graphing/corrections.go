package graphing

import (
	"fmt"
	"io"
	"strings"

	graph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Structs.

// GoalRulePair
type GoalRulePair struct {
	Goal *fi.Goal
	Rule *fi.Rule
}

// Functions.

// findPreTriggers extracts the trigger events
// that mark the transition from the precondition
// turning from false to true.
func (n *Neo4J) findPreTriggers(run uint) (map[*fi.Rule][]*GoalRulePair, error) {

	// Query precondition provenance of specified run
	// for event chains representing the following form:
	// aggregation rule, trigger goal, trigger rule.
	stmtTriggers, err := n.Conn1.PrepareNeo(`
		MATCH (a:Rule {run: {run}, condition: "pre"})-[*1]->(g:Goal {run: {run}, condition: "pre", condition_holds: false})-[*1]->(r:Rule {run: {run}, condition: "pre"})
		WHERE (:Goal {run: {run}, condition: "pre", condition_holds: true})-[*1]->(a)-[*1]->(g)-[*1]->(r)
		RETURN a AS aggregation, g AS goal, r AS rule;
    `)
	if err != nil {
		return nil, err
	}

	triggersRaw, err := stmtTriggers.QueryNeo(map[string]interface{}{
		"run": run,
	})
	if err != nil {
		return nil, err
	}

	// Prepare a map indexed by aggregation rule,
	// collecting all trigger goals and rules.
	triggers := make(map[*fi.Rule][]*GoalRulePair)

	for err == nil {

		var trigger []interface{}

		trigger, _, err = triggersRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, err
		} else if err == nil {

			// Type-assert the nodes into well-defined struct.
			agg := trigger[0].(graph.Node)
			goal := trigger[1].(graph.Node)
			rule := trigger[2].(graph.Node)

			// Parse parts that make up label of goal.
			goalLabel := strings.TrimLeft(goal.Properties["label"].(string), goal.Properties["table"].(string))
			goalLabel = strings.Trim(goalLabel, "()")
			goalLabelParts := strings.Split(goalLabel, ", ")

			aggregation := &fi.Rule{
				ID:    agg.Properties["id"].(string),
				Label: agg.Properties["label"].(string),
				Table: agg.Properties["table"].(string),
				Type:  agg.Properties["type"].(string),
			}

			if len(triggers[aggregation]) < 1 {
				triggers[aggregation] = make([]*GoalRulePair, 0, 4)
			}

			// Insert goal-rule pair into slice indexed
			// by aggregation rule.
			triggers[aggregation] = append(triggers[aggregation], &GoalRulePair{
				Goal: &fi.Goal{
					ID:        goal.Properties["id"].(string),
					Label:     goal.Properties["label"].(string),
					Table:     goal.Properties["table"].(string),
					Time:      goal.Properties["time"].(string),
					CondHolds: goal.Properties["condition_holds"].(bool),
					Receiver:  goalLabelParts[0],
				},
				Rule: &fi.Rule{
					ID:    rule.Properties["id"].(string),
					Label: rule.Properties["label"].(string),
					Table: rule.Properties["table"].(string),
					Type:  rule.Properties["type"].(string),
				},
			})
		}
	}

	err = triggersRaw.Close()
	if err != nil {
		return nil, err
	}

	err = stmtTriggers.Close()
	if err != nil {
		return nil, err
	}

	return triggers, nil
}

// findPostTriggers extracts the trigger events
// that mark the transition from the postcondition
// turning from false to true.
func (n *Neo4J) findPostTriggers(run uint) (map[*fi.Goal][]*fi.Rule, error) {

	// Query postcondition provenance of specified run
	// for pairs of trigger goal and trigger rule.
	stmtTriggers, err := n.Conn1.PrepareNeo(`
		MATCH (g:Goal {run: {run}, condition: "post", condition_holds: true})-[*1]->(r:Rule {run: {run}, condition: "post"})
		WHERE (:Rule {run: {run}, condition: "post"})-[*1]->(g)-[*1]->(r)-[*1]->(:Goal {run: {run}, condition: "post", condition_holds: false})-[*1]->(:Rule {run: {run}, condition: "post"})
		RETURN g AS goal, r AS rule;
    `)
	if err != nil {
		return nil, err
	}

	triggersRaw, err := stmtTriggers.QueryNeo(map[string]interface{}{
		"run": run,
	})
	if err != nil {
		return nil, err
	}

	// Prepare a map indexed by trigger goal,
	// collecting all trigger rules.
	triggers := make(map[*fi.Goal][]*fi.Rule)

	for err == nil {

		var trigger []interface{}

		trigger, _, err = triggersRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, err
		} else if err == nil {

			// Type-assert the nodes into well-defined struct.
			goal := trigger[0].(graph.Node)
			rule := trigger[1].(graph.Node)

			// Parse parts that make up label of goal.
			goalLabel := strings.TrimLeft(goal.Properties["label"].(string), goal.Properties["table"].(string))
			goalLabel = strings.Trim(goalLabel, "()")
			goalLabelParts := strings.Split(goalLabel, ", ")

			g := &fi.Goal{
				ID:        goal.Properties["id"].(string),
				Label:     goal.Properties["label"].(string),
				Table:     goal.Properties["table"].(string),
				Time:      goal.Properties["time"].(string),
				CondHolds: goal.Properties["condition_holds"].(bool),
				Receiver:  goalLabelParts[0],
			}

			if len(triggers[g]) < 1 {
				triggers[g] = make([]*fi.Rule, 0, 3)
			}

			// Insert rule into slice indexed by goal.
			triggers[g] = append(triggers[g], &fi.Rule{
				ID:    rule.Properties["id"].(string),
				Label: rule.Properties["label"].(string),
				Table: rule.Properties["table"].(string),
				Type:  rule.Properties["type"].(string),
			})
		}
	}

	err = triggersRaw.Close()
	if err != nil {
		return nil, err
	}

	err = stmtTriggers.Close()
	if err != nil {
		return nil, err
	}

	return triggers, nil
}

// GenerateCorrections extracts the triggering events required
// to achieve pre- and postcondition in the first (successful)
// run. We use this information in case the fault injector was
// able to inject a fault that caused the invariant to be violated
// in order to generate correction suggestions for how the system
// designers could strengthen the precondition to only fire when
// we are sure the postcondition holds as well.
func (n *Neo4J) GenerateCorrections() ([]string, error) {

	fmt.Printf("Running generation of suggestions for corrections (pre ~> post)... ")

	// Recs will contain our top-level recommendations.
	recs := make([]string, 0, 6)

	// Extract the precondition trigger event chains.
	preTriggers, err := n.findPreTriggers(0)
	if err != nil {
		return nil, err
	}

	// Extract the postcondition trigger event chains.
	postTriggers, err := n.findPostTriggers(0)
	if err != nil {
		return nil, err
	}

	// Prepare slice of strings representing the
	// compound of trigger rules required for firing
	// the respective aggregation rule.
	preTriggerRules := make(map[string]string)

	// Track per pre-rule if the nodes involved on
	// both sides, pre and post, differ. If so, we
	// have to take extra steps.
	differentNodes := make(map[string]map[string][]*fi.Goal)

	for preAgg := range preTriggers {

		differentNodes[preAgg.Table] = make(map[string][]*fi.Goal)

		for i := range preTriggers[preAgg] {

			if preTriggerRules[preAgg.Table] == "" {
				preTriggerRules[preAgg.Table] = fmt.Sprintf("%s(%s, ...) :- %s(%s, ...)", preAgg.Table, preTriggers[preAgg][i].Goal.Receiver, preTriggers[preAgg][i].Rule.Table, preTriggers[preAgg][i].Goal.Receiver)
			} else {
				preTriggerRules[preAgg.Table] = fmt.Sprintf("%s, %s(%s, ...)", preTriggerRules[preAgg.Table], preTriggers[preAgg][i].Rule.Table, preTriggers[preAgg][i].Goal.Receiver)
			}
		}
	}

	for preAgg := range preTriggers {

		for i := range preTriggers[preAgg] {

			for postGoal := range postTriggers {

				if preTriggers[preAgg][i].Goal.Receiver != postGoal.Receiver {

					if differentNodes[preAgg.Table][preTriggers[preAgg][i].Goal.Receiver] == nil {
						differentNodes[preAgg.Table][preTriggers[preAgg][i].Goal.Receiver] = make([]*fi.Goal, 0, 3)
					}

					differentNodes[preAgg.Table][preTriggers[preAgg][i].Goal.Receiver] = append(differentNodes[preAgg.Table][preTriggers[preAgg][i].Goal.Receiver], postGoal)
				}
			}
		}

		aggNew := preTriggerRules[preAgg.Table]

		if len(differentNodes[preAgg.Table]) == 0 {

			// The involved nodes for this precondition
			// rule and all postcondition rules to add are
			// the same ones. Thus, local order suffices.

			for postGoal := range postTriggers {
				aggNew = fmt.Sprintf("%s, %s(%s, ...)", aggNew, postGoal.Table, postGoal.Receiver)
			}
		} else {

			// At least one goal on the postcondition side
			// takes place on a different node than this
			// precondition's goal. We need communication.

			for pre := range differentNodes[preAgg.Table] {

				for post := range differentNodes[preAgg.Table][pre] {

					preNode := pre
					postNode := differentNodes[preAgg.Table][pre][post].Receiver
					postRule := differentNodes[preAgg.Table][pre][post].Table

					// Add the recommendation to integrate a message round
					// so that the receiver node in pre knows about the state.
					recs = append(recs, fmt.Sprintf("<code>%s</code> needs to know that <code>%s</code> has executed <code>%s</code>. Add:<br /> &nbsp; &nbsp; &nbsp; &nbsp; <code>ack_%s(%s, ...)@async :- %s(%s, ...), ...;</code>", preNode, postNode, postRule, postRule, preNode, postRule, postNode))

					// Also, add receipt of this message as dependency to
					// the updated precondition trigger.
					aggNew = fmt.Sprintf("%s, ack_%s(%s, sender=%s, ...)", aggNew, postRule, preNode, postNode)
				}
			}

			for i := range preTriggers[preAgg] {

				if preTriggers[preAgg][i].Rule.Type != "next" {

					// In case one of the rules underneath the aggregation
					// rule right above the triggering rules for the pre-
					// condition is not of type next (i.e., state-preserving),
					// we need to introduce a buffering scheme so that we do
					// not lose the state required for firing pre.

					rule := preTriggers[preAgg][i].Rule.Table
					node := preTriggers[preAgg][i].Goal.Receiver

					// Add the buffer_RULE construct as a suggestion.
					recs = append(recs, fmt.Sprintf("Precondition depends on timing of an onetime event. Make it persistent. Add:<br /> &nbsp; &nbsp; &nbsp; &nbsp; <code>buffer_%s(%s, ...) :- %s(%s, ...), ...;</code><br /> &nbsp; &nbsp; &nbsp; &nbsp; <code>buffer_%s(%s, ...)@next :- buffer_%s(%s, ...), ...;", rule, node, rule, node, rule, node, rule, node))

					// Update the new precondition trigger dependencies
					// by replacing the old rule with the new buffer_ rule.
					aggNew = strings.Replace(aggNew, fmt.Sprintf("%s(%s, ...)", rule, node), fmt.Sprintf("buffer_%s(%s, ...)", rule, node), -1)
				}
			}
		}

		// Finally, append the updated dependency rule
		// for firing the precondition to our recommendations.
		recs = append(recs, fmt.Sprintf("Change: <code>%s;</code> &nbsp; <i class = \"fas fa-long-arrow-alt-right\"></i> &nbsp; <code>%s;</code>", preTriggerRules[preAgg.Table], aggNew))
	}

	fmt.Printf("done\n\n")

	return recs, nil
}
