package graphing

import (
	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
)

// Structs.

// Neo4J
type Neo4J struct {
	Conn neo4j.Conn
}

// Functions.

// InitGraphDB
func (n *Neo4J) InitGraphDB(boltURI string) error {

	driver := neo4j.NewDriver()

	// Connect to bolt endpoint.
	c, err := driver.OpenNeo(boltURI)
	if err != nil {
		return err
	}

	n.Conn = c

	return nil
}

// LoadNaiveProv
func (n *Neo4J) LoadNaiveProv() error {

	return nil
}
