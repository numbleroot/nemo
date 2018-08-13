package graphing

import (
	"fmt"
	"io"
	"strconv"
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

// Dependency
type Dependency struct {
	Rule string
	Time uint
}

// Functions.

// findAsyncEvents
func (n *Neo4J) findAsyncEvents(failedRun uint, msgs []*fi.Message) ([]*GoalRulePair, []*GoalRulePair, error) {

	diffRunID := 2000 + failedRun

	// Determine if there is non-triviality (i.e., async events)
	// in the failed run's precondition provenance.
	stmtPreAsync, err := n.Conn1.PrepareNeo(`
		MATCH path = (root {run: {run}, condition: "pre"})-[*0..]->(r1:Rule {run: {run}, condition: "pre", type: "async"})-[*0..]->(r2:Rule {run: {run}, condition: "pre", type: "async"})
		WHERE NOT ()-->(root)
		WITH r2, collect(r1) AS history

		MATCH (g:Goal)-[*1]->(r2)
		WITH r2, g, history
		RETURN r2 AS rule, g AS goal, filter(_ IN history WHERE size(history) = 1) AS history;
    `)
	if err != nil {
		return nil, nil, err
	}

	preAsyncsRaw, err := stmtPreAsync.QueryNeo(map[string]interface{}{
		"run": failedRun,
	})
	if err != nil {
		return nil, nil, err
	}

	preAsyncs := make([]*GoalRulePair, 0, 5)

	for err == nil {

		var preAsync []interface{}

		preAsync, _, err = preAsyncsRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, nil, err
		} else if err == nil {

			history := preAsync[2].([]interface{})

			// We only consider results that are not subsumed
			// by other results.
			if len(history) == 1 {

				var sender string

				// Type-assert the two nodes into well-defined struct.
				rule := preAsync[0].(graph.Node)
				goal := preAsync[1].(graph.Node)

				// Parse parts that make up label of goal.
				goalLabel := strings.TrimLeft(goal.Properties["label"].(string), goal.Properties["table"].(string))
				goalLabel = strings.Trim(goalLabel, "()")
				goalLabelParts := strings.Split(goalLabel, ", ")

				for m := range msgs {

					t, err := strconv.ParseUint(goal.Properties["time"].(string), 10, 32)
					if err != nil {
						return nil, nil, err
					}

					if (msgs[m].Content == rule.Properties["label"].(string)) &&
						(msgs[m].RecvNode == goalLabelParts[0]) &&
						(msgs[m].RecvTime == uint(t)) {
						sender = msgs[m].SendNode
					}
				}

				// Append to slice of correction pairs.
				preAsyncs = append(preAsyncs, &GoalRulePair{
					Goal: &fi.Goal{
						ID:        goal.Properties["id"].(string),
						Label:     goal.Properties["label"].(string),
						Table:     goal.Properties["table"].(string),
						Time:      goal.Properties["time"].(string),
						CondHolds: goal.Properties["condition_holds"].(bool),
						Sender:    sender,
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
	}

	err = preAsyncsRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	err = stmtPreAsync.Close()
	if err != nil {
		return nil, nil, err
	}

	// Determine if there is non-triviality (i.e., async events)
	// in differential postcondition provenance.
	stmtDiffAsync, err := n.Conn1.PrepareNeo(`
		MATCH path = (root {run: {run}, condition: "post"})-[*0..]->(r1:Rule {run: {run}, condition: "post", type: "async"})-[*0..]->(r2:Rule {run: {run}, condition: "post", type: "async"})
		WHERE NOT ()-->(root)
		WITH r2, collect(r1) AS history

		MATCH (g:Goal)-[*1]->(r2)
		WITH r2, g, history
		RETURN r2 AS rule, g AS goal, filter(_ IN history WHERE size(history) = 1) AS history;
    `)
	if err != nil {
		return nil, nil, err
	}

	diffAsyncsRaw, err := stmtDiffAsync.QueryNeo(map[string]interface{}{
		"run": diffRunID,
	})
	if err != nil {
		return nil, nil, err
	}

	diffAsyncs := make([]*GoalRulePair, 0, 5)

	for err == nil {

		var diffAsync []interface{}

		diffAsync, _, err = diffAsyncsRaw.NextNeo()
		if err != nil && err != io.EOF {
			return nil, nil, err
		} else if err == nil {

			history := diffAsync[2].([]interface{})

			if len(history) == 1 {

				// Type-assert the two nodes into well-defined struct.
				rule := diffAsync[0].(graph.Node)
				goal := diffAsync[1].(graph.Node)

				// Parse parts that make up label of goal.
				goalLabel := strings.TrimLeft(goal.Properties["label"].(string), goal.Properties["table"].(string))
				goalLabel = strings.Trim(goalLabel, "()")
				goalLabelParts := strings.Split(goalLabel, ", ")

				// Append to slice of correction pairs.
				diffAsyncs = append(diffAsyncs, &GoalRulePair{
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
	}

	err = diffAsyncsRaw.Close()
	if err != nil {
		return nil, nil, err
	}

	err = stmtDiffAsync.Close()
	if err != nil {
		return nil, nil, err
	}

	return preAsyncs, diffAsyncs, nil
}

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

	recs = append(recs, "A fault occurred. Let's try making the protocol correct first. Change:")

	// Prepare slice of strings representing the
	// compound of trigger rules required for firing
	// the respective aggregation rule.
	preTriggerRules := make(map[string]string)

	for preAgg := range preTriggers {

		// Track which rules we already considered for
		// this aggregation rule.
		considered := make(map[string]bool)

		for i := range preTriggers[preAgg] {

			// Only add next rule if we have not yet considered it.
			if !considered[preTriggers[preAgg][i].Rule.Table] {

				if preTriggerRules[preAgg.Table] == "" {
					preTriggerRules[preAgg.Table] = fmt.Sprintf("%s(%s, ...) :- %s(%s, ...)", preAgg.Table, preTriggers[preAgg][i].Goal.Receiver, preTriggers[preAgg][i].Rule.Table, preTriggers[preAgg][i].Goal.Receiver)
				} else {
					preTriggerRules[preAgg.Table] = fmt.Sprintf("%s, %s(%s, ...)", preTriggerRules[preAgg.Table], preTriggers[preAgg][i].Rule.Table, preTriggers[preAgg][i].Goal.Receiver)
				}

				// After inclusion, mark trigger rule as considered.
				considered[preTriggers[preAgg][i].Rule.Table] = true
			}
		}

		// Build the correction suggestion rule.
		aggNew := preTriggerRules[preAgg.Table]

		for postGoal := range postTriggers {

			for i := range postTriggers[postGoal] {

				if postGoal.Receiver == preTriggers[preAgg][0].Goal.Receiver {
					aggNew = fmt.Sprintf("%s, %s(%s, ...)", aggNew, postTriggers[postGoal][i].Table, postGoal.Receiver)
				} else {

					// There is network communication required.
					// Tell to system designers.
					recs = append(recs, fmt.Sprintf("<code>%s</code> needs to know that <code>%s</code> has executed <code>%s</code>. Add:<br /> &nbsp; &nbsp; &nbsp; &nbsp; <code>ack_%s(%s, ...)@async :- %s(%s, ...), ...;</code>", preTriggers[preAgg][0].Goal.Receiver, postGoal.Receiver, postTriggers[postGoal][i].Table, postTriggers[postGoal][i].Table, preTriggers[preAgg][0].Goal.Receiver, postTriggers[postGoal][i].Table, postGoal.Receiver))

					aggNew = fmt.Sprintf("%s, ack_%s(%s, ...)", aggNew, postTriggers[postGoal][i].Table, preTriggers[preAgg][0].Goal.Receiver)
				}
			}
		}

		// Append our recommendation.
		recs = append(recs, fmt.Sprintf("<code>%s;</code> &nbsp; <i class = \"fas fa-long-arrow-alt-right\"></i> &nbsp; <code>%s;</code>", preTriggerRules[preAgg.Table], aggNew))
	}

	return recs, nil
}

// GenerateCorrections
func (n *Neo4J) GenerateCorrectionsOld(failedRuns []uint, msgs [][]*fi.Message) ([][]string, error) {

	fmt.Printf("Running generation of suggestions for corrections (pre ~> post)... ")

	// Prepare final slices to return.
	allCorrections := make([][]string, len(failedRuns))

	for i := range failedRuns {

		// Create local slices.
		corrections := make([]string, 0, 6)

		// Retrieve non-trivial events in pre and diffprov post.
		preAsyncs, diffAsyncs, err := n.findAsyncEvents(failedRuns[i], msgs[i])
		if err != nil {
			return nil, err
		}

		if len(preAsyncs) < 1 {

			// No message passing events in precondition provenance.

			if len(diffAsyncs) < 1 {
				// No message passing events in differential postcondition provenance.
				// TODO: Discuss this and remove last correction appending.
				corrections = append(corrections, "<pre>[Precondition]</pre> No message passing events required.")
				corrections = append(corrections, "<pre>[Postcondition]</pre> No message passing events left.")
				corrections = append(corrections, "Yet we saw a fault occuring. <span style = \"font-weight: bold;\">Discuss: What are the use cases?</span>")
			} else {

				diffAsyncsLabel := fmt.Sprintf("<code>%s</code> @ <code>%s</code>", diffAsyncs[0].Rule.Label, diffAsyncs[0].Goal.Time)
				for j := 1; j < len(diffAsyncs); j++ {
					diffAsyncsLabel = fmt.Sprintf("%s, <code>%s</code> @ <code>%s</code>", diffAsyncsLabel, diffAsyncs[j].Rule.Label, diffAsyncs[j].Goal.Time)
				}

				// At least one message passing event in differential postcondition provenance.
				corrections = append(corrections, "<pre>[Precondition]</pre> No message passing events required.")
				corrections = append(corrections, fmt.Sprintf("<pre>[Postcondition]</pre> Latest message passing events still missing: %s", diffAsyncsLabel))
				corrections = append(corrections, "<span style = \"font-weight: bold;\">Suggestion: Introduce more fault-tolerance through replication and retries.</span>")
			}
		} else {

			// At least one message passing event in precondition provenance.

			preAsyncsLabel := fmt.Sprintf("<code>%s</code> @ <code>%s</code>", preAsyncs[0].Rule.Label, preAsyncs[0].Goal.Time)
			for j := 1; j < len(preAsyncs); j++ {
				preAsyncsLabel = fmt.Sprintf("%s, <code>%s</code> @ <code>%s</code>", preAsyncsLabel, preAsyncs[j].Rule.Label, preAsyncs[j].Goal.Time)
			}

			if len(diffAsyncs) < 1 {
				// No message passing events in differential postcondition provenance.
				// TODO: Discuss this and remove last correction appending.
				corrections = append(corrections, fmt.Sprintf("<pre>[Precondition]</pre> Latest message passing events required: %s", preAsyncsLabel))
				corrections = append(corrections, "<pre>[Postcondition]</pre> No message passing events left.")
				corrections = append(corrections, "Yet we saw a fault occuring. <span style = \"font-weight: bold;\">Discuss: What are the use cases?</span>")
			} else {

				diffAsyncsLabel := fmt.Sprintf("<code>%s</code> @ <code>%s</code>", diffAsyncs[0].Rule.Label, diffAsyncs[0].Goal.Time)
				for j := 1; j < len(diffAsyncs); j++ {
					diffAsyncsLabel = fmt.Sprintf("%s, <code>%s</code> @ <code>%s</code>", diffAsyncsLabel, diffAsyncs[j].Rule.Label, diffAsyncs[j].Goal.Time)
				}

				// At least one message passing event in differential postcondition provenance.
				corrections = append(corrections, fmt.Sprintf("<pre>[Precondition]</pre> Latest message passing events required: %s", preAsyncsLabel))
				corrections = append(corrections, fmt.Sprintf("<pre>[Postcondition]</pre> Latest message passing events still missing: %s", diffAsyncsLabel))

				for j := range preAsyncs {

					updDeps := make(map[string]Dependency)

					for d := range diffAsyncs {

						// Check if we have to suggest introduction of
						// intermediate round-trips because the sender of
						// the pre async event is a different one from the
						// diff-post one.
						if preAsyncs[j].Goal.Sender != diffAsyncs[d].Goal.Receiver {

							t, err := strconv.ParseUint(diffAsyncs[d].Goal.Time, 10, 32)
							if err != nil {
								return nil, err
							}

							// Create new internal rule.
							intAckRule := fmt.Sprintf("int_ack_%s(%s, node, ...)", diffAsyncs[d].Goal.Table, preAsyncs[j].Goal.Sender)

							// If so, add a suggestion for an internal
							// acknowledgement round to pre async receiver.
							corrections = append(corrections, fmt.Sprintf("<span style = \"font-weight: bold;\">Suggestion:</span> <code>%s</code> needs to know that <code>%s</code> received <code>%s</code>.<br /> &nbsp; &nbsp; &nbsp; &nbsp; Add internal acknowledgement: <code>%s@async :- %s(node, ...)</code> @ <code>%s</code>;", preAsyncs[j].Goal.Sender, diffAsyncs[d].Goal.Receiver, diffAsyncs[d].Goal.Label, intAckRule, diffAsyncs[d].Rule.Table, diffAsyncs[d].Goal.Time))

							// Add intermediate ack to dependencies.
							updDeps[diffAsyncs[d].Rule.Label] = Dependency{
								Rule: intAckRule,
								Time: (uint(t) + 1),
							}
						} else {

							t, err := strconv.ParseUint(diffAsyncs[d].Goal.Time, 10, 32)
							if err != nil {
								return nil, err
							}

							// Otherwise, add the original diffprov rule.
							updDeps[diffAsyncs[d].Rule.Label] = Dependency{
								Rule: diffAsyncs[d].Rule.Label,
								Time: uint(t),
							}
						}
					}

					var maxTime uint = 0
					var updDiffAsyncsLabel string
					for d := range updDeps {

						if updDeps[d].Time > maxTime {
							maxTime = updDeps[d].Time
						}

						if updDiffAsyncsLabel == "" {
							updDiffAsyncsLabel = fmt.Sprintf("<code>%s</code> @ <code>%d</code>", updDeps[d].Rule, updDeps[d].Time)
						} else {
							updDiffAsyncsLabel = fmt.Sprintf("%s, <code>%s</code> @ <code>%d</code>", updDiffAsyncsLabel, updDeps[d].Rule, updDeps[d].Time)
						}
					}

					// Determine dependency suggestions for identified pre rules.
					corrections = append(corrections, fmt.Sprintf("<span style = \"font-weight: bold;\">Suggestion:</span> Augment the conditions under which <code>%s</code> fires:<br /> &nbsp; &nbsp; &nbsp; &nbsp; <code>%s(%s, ...)@async :- </code>%s, &nbsp; <code>EXISTING_DEPENDENCIES</code>;", preAsyncs[j].Rule.Label, preAsyncs[j].Rule.Label, preAsyncs[j].Goal.Receiver, updDiffAsyncsLabel))

					// Determine the updated (delayed) time of victory declaration.
					corrections = append(corrections, fmt.Sprintf("<span style = \"font-weight: bold;\">Timing:</span> Earliest time for safely firing <code>%s</code>: <code>%d</code>", preAsyncs[j].Rule.Label, maxTime))
				}
			}
		}

		if len(corrections) == 0 {
			corrections = append(corrections, "No correction suggestions to make!")
		}

		allCorrections[i] = corrections
	}

	fmt.Printf("done\n\n")

	return allCorrections, nil
}
