package graphing

import (
	"fmt"
	"strings"
	"time"

	"os/exec"

	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
	fi "github.com/numbleroot/nemo/faultinjectors"
)

// Functions.

// InitGraphDB
func (n *Neo4J) InitGraphDB(boltURI string, runs []*fi.Run) error {

	// Run the docker start command.
	fmt.Printf("Starting docker containers... ")
	cmd := exec.Command("sudo", "docker-compose", "-f", "docker-compose.yml", "up", "-d")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if !strings.Contains(string(out), "done") {
		return fmt.Errorf("Wrong return value from docker-compose up command: %s", out)
	}
	fmt.Printf("done\n")

	// Wait long enough for graph database to be up.
	time.Sleep(10 * time.Second)

	driver := neo4j.NewDriver()

	// Connect to bolt endpoint.
	c1, err := driver.OpenNeo(boltURI)
	if err != nil {
		return err
	}

	c2, err := driver.OpenNeo(boltURI)
	if err != nil {
		return err
	}

	n.Conn1 = c1
	n.Conn2 = c2
	n.Runs = runs

	fmt.Println()

	return nil
}

// CloseDB properly shuts down the Neo4J connection.
func (n *Neo4J) CloseDB() error {

	err := n.Conn1.Close()
	if err != nil {
		return err
	}

	err = n.Conn2.Close()
	if err != nil {
		return err
	}

	/*

		time.Sleep(2 * time.Second)

		// Shut down docker container.
		fmt.Printf("Shutting down docker containers... ")
		cmd := exec.Command("sudo", "docker-compose", "-f", "docker-compose.yml", "down")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		if !strings.Contains(string(out), "done") {
			return fmt.Errorf("Wrong return value from docker-compose down command: %s", out)
		}
		fmt.Printf("done\n")

	*/

	return nil
}
