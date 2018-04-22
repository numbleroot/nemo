package graphing

import (
	"fmt"
	"strings"
	"time"

	"os/exec"

	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
	"github.com/numbleroot/nemo/faultinjectors"
)

// Structs.

// Neo4J
type Neo4J struct {
	Conn neo4j.Conn
}

// Functions.

// InitGraphDB
func (n *Neo4J) InitGraphDB(boltURI string) error {

	// Run the docker start command.
	cmd := exec.Command("sudo", "docker-compose", "-f", "docker-compose.yml", "up", "-d")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if !strings.Contains(string(out), "done") {
		return fmt.Errorf("Wrong return value from docker-compose up command: %s", out)
	}

	// Wait long enough for graph database to be up.
	time.Sleep(5 * time.Second)

	driver := neo4j.NewDriver()

	// Connect to bolt endpoint.
	c, err := driver.OpenNeo(boltURI)
	if err != nil {
		return err
	}

	n.Conn = c

	return nil
}

// CloseDB properly shuts down the Neo4J connection.
func (n *Neo4J) CloseDB() error {

	err := n.Conn.Close()
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	// Shut down docker container.
	cmd := exec.Command("sudo", "docker-compose", "-f", "docker-compose.yml", "down")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if !strings.Contains(string(out), "done") {
		return fmt.Errorf("Wrong return value from docker-compose down command: %s", out)
	}

	return nil
}

// LoadNaiveProv
func (n *Neo4J) LoadNaiveProv(runs []*faultinjectors.Run) error {

	for i := range runs {

		stmtGoal, err := n.Conn.PrepareNeo("CREATE (n:Goal {run: {run}, condition: {condition}, label: {label}, table: {table}, type: {type}})")
		if err != nil {
			return err
		}

		for j := range runs[i].PreProv.Goals {

			// Create a goal node.
			_, err := stmtGoal.ExecNeo(map[string]interface{}{
				"run":       runs[i].Iteration,
				"condition": "pre",
				"label":     runs[i].PreProv.Goals[j].Label,
				"table":     runs[i].PreProv.Goals[j].Table,
				"type":      "single",
			})
			if err != nil {
				return err
			}
		}

		err = stmtGoal.Close()
		if err != nil {
			return err
		}

		stmtRule, err := n.Conn.PrepareNeo("CREATE (n:Rule {run: {run}, condition: {condition}, label: {label}, table: {table}, type: {type}})")
		if err != nil {
			return err
		}

		for j := range runs[i].PreProv.Rules {

			// Create a rule node.
			_, err := stmtRule.ExecNeo(map[string]interface{}{
				"run":       runs[i].Iteration,
				"condition": "pre",
				"label":     runs[i].PreProv.Rules[j].Label,
				"table":     runs[i].PreProv.Rules[j].Table,
				"type":      "single",
			})
			if err != nil {
				return err
			}
		}

		err = stmtRule.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
