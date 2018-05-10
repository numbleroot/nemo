package faultinjectors

// Structs.

// CrashFailure
type CrashFailure struct {
	Node string `json:"node"`
	Time uint   `json:"time"`
}

// MessageLoss
type MessageLoss struct {
	From string `json:"from"`
	To   string `json:"to"`
	Time uint   `json:"time"`
}

// FailureSpec
type FailureSpec struct {
	EOT        uint            `json:"eot"`
	EFF        uint            `json:"eff"`
	MaxCrashes uint            `json:"maxCrashes"`
	Nodes      *[]string       `json:"nodes"`
	Crashes    *[]CrashFailure `json:"crashes"`
	Omissions  *[]MessageLoss  `json:"omissions"`
}

// Model
type Model struct {
	Tables map[string][][]string `json:"tables"`
}

// Message
type Message struct {
	Content  string `json:"table"`
	SendNode string `json:"from"`
	RecvNode string `json:"to"`
	SendTime uint   `json:"sendTime"`
	RecvTime uint   `json:"receiveTime"`
}

// Goal
type Goal struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Table     string `json:"table"`
	Time      string `json:"time"`
	CondHolds bool   `json:"conditionHolds,omitempty"`
}

// Rule
type Rule struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Table string `json:"table"`
	Type  string `json:"type"`
}

// Edge
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ProvData
type ProvData struct {
	Goals []Goal `json:"goals"`
	Rules []Rule `json:"rules"`
	Edges []Edge `json:"edges"`
}

// Missing
type Missing struct {
	Rule  *Rule
	Goals []*Goal
}

// Correction
type Correction struct {
	PreRule  Rule `json:"preRule"`
	PostRule Rule `json:"postRule"`
}

// Run
type Run struct {
	Iteration        uint            `json:"iteration"`
	Status           string          `json:"status"`
	FailureSpec      *FailureSpec    `json:"failureSpec"`
	Model            *Model          `json:"model"`
	Messages         []*Message      `json:"messages"`
	PreProv          *ProvData       `json:"preProv,omitempty"`
	TimePreHolds     map[string]bool `json:"timePreHolds,omitempty"`
	PostProv         *ProvData       `json:"postProv,omitempty"`
	TimePostHolds    map[string]bool `json:"timePostHolds,omitempty"`
	MissingEvents    *Missing        `json:"missingEvents,omitempty"`
	Corrections      []string        `json:"corrections,omitempty"`
	CorrectionsPairs []*Correction   `json:"correctionsPairs,omitempty"`
}

// Molly
type Molly struct {
	Run              string
	OutputDir        string
	Runs             []*Run
	RunsIters        []uint
	SuccessRunsIters []uint
	FailedRunsIters  []uint
}
