package protocol

import "fmt"

// Issue and Comment are the complete non-recursive accepted records. Relationships
// hold identities only; subsequent resource slices share these public types.
type Issue struct {
	SchemaVersion int      `json:"schema_version"`
	ID            string   `json:"id"`
	Revision      int64    `json:"revision"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	State         string   `json:"state"`
	Labels        []string `json:"labels"`
	Assignee      *string  `json:"assignee"`
	ProjectID     string   `json:"project_id"`
	ParentID      *string  `json:"parent_id"`
	ChildIDs      []string `json:"child_ids"`
	CommentIDs    []string `json:"comment_ids"`
	Blocks        []string `json:"blocks"`
	BlockedBy     []string `json:"blocked_by"`
	Related       []string `json:"related"`
}
type Comment struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Revision      int64  `json:"revision"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	Body          string `json:"body"`
	Author        string `json:"author"`
	IssueID       string `json:"issue_id"`
}

func ResourceType(t string) bool { return t == "projects" || t == "issues" || t == "comments" }
func emptySet(s ProjectSet) bool {
	return s.Title == nil && s.Description == nil && s.RepositoryRefs == nil && s.Body == nil && s.State == nil && s.Labels == nil && s.Assignee == nil && s.Author == nil && s.ProjectID == nil && s.IssueID == nil && s.ParentID == nil
}
func validateOperationFields(o Operation) error {
	s := o.Set
	if o.Type != "issues" || o.Kind != "update" {
		if len(o.Add.Blocks)+len(o.Remove.Blocks)+len(o.Add.BlockedBy)+len(o.Remove.BlockedBy)+len(o.Add.Related)+len(o.Remove.Related) > 0 {
			return fmt.Errorf("inapplicable relationships")
		}
	}
	for _, pair := range [][2][]string{{o.Add.Blocks, o.Remove.Blocks}, {o.Add.BlockedBy, o.Remove.BlockedBy}, {o.Add.Related, o.Remove.Related}} {
		for _, a := range pair[0] {
			if !ValidUUID(a) {
				return fmt.Errorf("invalid relationship ID")
			}
			for _, b := range pair[1] {
				if a == b {
					return fmt.Errorf("contradictory relationship changes")
				}
			}
		}
		for _, b := range pair[1] {
			if !ValidUUID(b) {
				return fmt.Errorf("invalid relationship ID")
			}
		}
	}
	bad := false
	switch o.Type {
	case "projects":
		bad = s.Body != nil || s.State != nil || s.Labels != nil || s.Assignee != nil || s.Author != nil || s.ProjectID != nil || s.IssueID != nil || s.ParentID != nil || len(o.Add.Labels)+len(o.Remove.Labels) > 0
	case "issues":
		bad = s.Description != nil || s.RepositoryRefs != nil || s.Author != nil || s.IssueID != nil || len(o.Add.RepositoryRefs)+len(o.Remove.RepositoryRefs) > 0 || (o.Kind == "update" && s.ProjectID != nil)
	case "comments":
		bad = s.Title != nil || s.Description != nil || s.RepositoryRefs != nil || s.State != nil || s.Labels != nil || s.Assignee != nil || s.ProjectID != nil || s.ParentID != nil || len(o.Add.RepositoryRefs)+len(o.Remove.RepositoryRefs)+len(o.Add.Labels)+len(o.Remove.Labels) > 0 || (o.Kind == "update" && s.IssueID != nil)
	}
	if bad {
		return fmt.Errorf("inapplicable fields for %s %s", o.Type, o.Kind)
	}
	if s.Labels != nil && len(o.Add.Labels)+len(o.Remove.Labels) > 0 {
		return fmt.Errorf("label replacement conflicts with member changes")
	}
	for _, a := range o.Add.Labels {
		for _, b := range o.Remove.Labels {
			if a == b {
				return fmt.Errorf("contradictory label changes")
			}
		}
	}
	return nil
}

// ValidateResourceShape is presence-aware even for empty inapplicable fields.
func (p ProjectInput) ValidateResourceShape(resource, kind string) error {
	if resource == "projects" {
		return p.ValidateShape(kind)
	}
	if kind != "link" && kind != "unlink" && len(p.Add.Blocks)+len(p.Add.BlockedBy)+len(p.Add.Related)+len(p.Remove.Blocks)+len(p.Remove.BlockedBy)+len(p.Remove.Related) > 0 {
		return fmt.Errorf("relationship changes use link or unlink")
	}
	allowed := map[string]bool{}
	if kind == "link" || kind == "unlink" {
		for _, k := range []string{"from", "to", "relation"} {
			allowed[k] = true
		}
	} else if kind == "create" {
		for _, k := range []string{"id", "content"} {
			allowed[k] = true
		}
		if resource == "issues" {
			for _, k := range []string{"project", "parent", "title", "state", "labels", "assignee"} {
				allowed[k] = true
			}
		} else {
			allowed["issue"] = true
			allowed["author"] = true
		}
	} else if kind == "close" || kind == "reopen" {
		allowed["target"] = true
		allowed["expected_revision"] = true
	} else {
		for _, k := range []string{"target", "expected_revision", "set", "clear"} {
			allowed[k] = true
		}
		if resource == "issues" {
			allowed["add"] = true
			allowed["remove"] = true
			allowed["parent"] = true
		}
	}
	for k := range p.Fields {
		if !allowed[k] {
			return fmt.Errorf("%s is inapplicable to %s %s", k, resource, kind)
		}
	}
	return nil
}
