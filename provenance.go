package main

// Structs.

// Node
type Node struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Table string `json:"table"`
}

// Edge
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ProvData
type ProvData struct {
	Goals []Node `json:"goals"`
	Rules []Node `json:"rules"`
	Edges []Edge `json:"edges"`
}
