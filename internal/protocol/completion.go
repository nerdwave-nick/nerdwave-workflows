package protocol

// Completion contains only identifiers and display metadata. Keep this an
// explicit projection: embedding a record would expose its content to reads
// that deliberately do not require a connected client.
type Completion struct {
	Value        string `json:"value"`
	ID           string `json:"id"`
	Title        string `json:"title,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	ProjectTitle string `json:"project_title,omitempty"`
	IssueID      string `json:"issue_id,omitempty"`
	State        string `json:"state,omitempty"`
}

type Completions struct {
	ServiceID string       `json:"service_id"`
	APIMajor  int          `json:"api_major"`
	Items     []Completion `json:"items"`
	HasMore   bool         `json:"has_more"`
}
