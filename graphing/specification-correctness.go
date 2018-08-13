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

				// Provide raw name excluding "_provX" ending.
				ruleLabelParts := strings.Split(rule.Properties["label"].(string), "_")
				rule.Properties["label"] = strings.Join(ruleLabelParts[:(len(ruleLabelParts)-1)], "_")

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

				// Provide raw name excluding "_provX" ending.
				ruleLabelParts := strings.Split(rule.Properties["label"].(string), "_")
				rule.Properties["label"] = strings.Join(ruleLabelParts[:(len(ruleLabelParts)-1)], "_")

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

// findTriggerEvents extracts the trigger events
// that mark the transition from passed condition
// being false to being true.
func (n *Neo4J) findTriggerEvents(run uint, condition string) (map[*fi.Rule][]*GoalRulePair, error) {

	// Query run and condition provenance specified via function
	// arguments for event chains representing the following form:
	// aggregation rule, trigger goal, trigger rule.
	stmtTriggers, err := n.Conn1.PrepareNeo(`
		MATCH (a:Rule {run: {run}, condition: {condition}})-[*1]->(g:Goal {run: {run}, condition: {condition}, condition_holds: true})-[*1]->(r:Rule {run: {run}, condition: {condition}})
		WHERE (r)-[*1]->(:Goal {run: {run}, condition: {condition}, condition_holds: false})
		RETURN a AS aggregation, g AS goal, r AS rule;
    `)
	if err != nil {
		return nil, err
	}

	triggersRaw, err := stmtTriggers.QueryNeo(map[string]interface{}{
		"run":       run,
		"condition": condition,
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

			// Type-assert the three nodes into well-defined struct.
			agg := trigger[0].(graph.Node)
			goal := trigger[1].(graph.Node)
			rule := trigger[2].(graph.Node)

			// Extract raw name excluding "_provX" ending.
			aggLabelParts := strings.Split(agg.Properties["label"].(string), "_")
			agg.Properties["table"] = strings.Join(aggLabelParts[:(len(aggLabelParts)-1)], "_")

			// Parse parts that make up label of goal.
			goalLabel := strings.TrimLeft(goal.Properties["label"].(string), goal.Properties["table"].(string))
			goalLabel = strings.Trim(goalLabel, "()")
			goalLabelParts := strings.Split(goalLabel, ", ")

			// Extract raw name excluding "_provX" ending.
			ruleLabelParts := strings.Split(rule.Properties["label"].(string), "_")
			rule.Properties["table"] = strings.Join(ruleLabelParts[:(len(ruleLabelParts)-1)], "_")

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

	// Extract the precondition trigger event chains:
	// aggregation rule -> trigger goals -> trigger rules.
	preTriggers, err := n.findTriggerEvents(0, "pre")
	if err != nil {
		return nil, err
	}

	// Extract the postcondition trigger event chains:
	// aggregation rule -> trigger goals -> trigger rules.
	postTriggers, err := n.findTriggerEvents(0, "post")
	if err != nil {
		return nil, err
	}

	// Create string representations of the trigger rules
	// for establishing the postcondition.
	postTriggerRules := make([]string, len(postTriggers))

	for agg := range postTriggers {

		u := 0

		for i := range postTriggers[agg] {

			if postTriggerRules[u] == "" {
				postTriggerRules[u] = fmt.Sprintf("%s(...)", postTriggers[agg][i].Rule.Table)
			} else {
				postTriggerRules[u] = fmt.Sprintf("%s, %s(...)", postTriggerRules[u], postTriggers[agg][i].Rule.Table)
			}
		}

		u++
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
					preTriggerRules[preAgg.Table] = fmt.Sprintf("%s(...) := %s(...)", preAgg.Table, preTriggers[preAgg][i].Rule.Table)
				} else {
					preTriggerRules[preAgg.Table] = fmt.Sprintf("%s, %s(...)", preTriggerRules[preAgg.Table], preTriggers[preAgg][i].Rule.Table)
				}

				// After inclusion, mark trigger rule as considered.
				considered[preTriggers[preAgg][i].Rule.Table] = true
			}
		}

		// Build the correction suggestion rule.
		aggNew := preTriggerRules[preAgg.Table]

		for postAgg := range postTriggers {

			for i := range postTriggers[postAgg] {

				if !considered[postTriggers[postAgg][i].Rule.Table] {
					aggNew = fmt.Sprintf("%s, %s(...)", aggNew, postTriggers[postAgg][i].Rule.Table)
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
							corrections = append(corrections, fmt.Sprintf("<span style = \"font-weight: bold;\">Suggestion:</span> <code>%s</code> needs to know that <code>%s</code> received <code>%s</code>.<br /> &nbsp; &nbsp; &nbsp; &nbsp; Add internal acknowledgement: <code>%s@async :- %s(node, ...)</code> @ <code>%s</code>;", preAsyncs[j].Goal.Sender, diffAsyncs[d].Goal.Receiver, diffAsyncs[d].Goal.Label, intAckRule, diffAsyncs[d].Rule.Label, diffAsyncs[d].Goal.Time))

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
