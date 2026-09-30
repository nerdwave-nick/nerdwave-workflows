package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type DurableActor struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}
type ObjectRef struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	ExpectedRevision *int64 `json:"expected_revision"`
}
type ProjectSet struct {
	Title          *string   `json:"title,omitempty"`
	Description    *string   `json:"description,omitempty"`
	RepositoryRefs *[]string `json:"repository_refs,omitempty"`
	Body           *string   `json:"body,omitempty"`
	State          *string   `json:"state,omitempty"`
	Labels         *[]string `json:"labels,omitempty"`
	Assignee       **string  `json:"assignee,omitempty"`
	Author         *string   `json:"author,omitempty"`
	ProjectID      *string   `json:"project_id,omitempty"`
	IssueID        *string   `json:"issue_id,omitempty"`
	ParentID       **string  `json:"parent_id,omitempty"`
}
type ProjectMembers struct {
	Blocks         []string `json:"blocks,omitempty"`
	BlockedBy      []string `json:"blocked_by,omitempty"`
	Related        []string `json:"related,omitempty"`
	RepositoryRefs []string `json:"repository_refs,omitempty"`
	Labels         []string `json:"labels,omitempty"`
}
type Operation struct {
	Type   string         `json:"type"`
	ID     string         `json:"id"`
	Kind   string         `json:"kind"`
	Set    ProjectSet     `json:"set"`
	Add    ProjectMembers `json:"add"`
	Remove ProjectMembers `json:"remove"`
}
type Intent struct {
	SchemaVersion int          `json:"schema_version"`
	Operation     string       `json:"operation"`
	ServiceID     string       `json:"service_id"`
	Actor         DurableActor `json:"actor"`
	Operations    []Operation  `json:"operations"`
	Targets       []ObjectRef  `json:"targets"`
	Guards        []ObjectRef  `json:"guards,omitempty"`
	Force         bool         `json:"force,omitempty"`
}
type DurableRequest struct {
	SchemaVersion int          `json:"schema_version"`
	Intent        Intent       `json:"intent"`
	RequestHash   string       `json:"request_hash"`
	Claims        []ClaimToken `json:"claims,omitempty"`
}
type ProjectInput struct {
	From             string          `json:"from,omitempty"`
	To               []string        `json:"to,omitempty"`
	Relation         string          `json:"relation,omitempty"`
	Fields           map[string]bool `json:"-"`
	Project          string          `json:"project,omitempty"`
	Parent           *string         `json:"parent,omitempty"`
	Issue            string          `json:"issue,omitempty"`
	State            *string         `json:"state,omitempty"`
	Labels           []string        `json:"labels,omitempty"`
	Assignee         *string         `json:"assignee,omitempty"`
	Author           *string         `json:"author,omitempty"`
	ID               string          `json:"id,omitempty"`
	Target           string          `json:"target,omitempty"`
	Title            *string         `json:"title,omitempty"`
	Content          *string         `json:"content,omitempty"`
	RepositoryRefs   []string        `json:"repository_refs,omitempty"`
	ExpectedRevision *int64          `json:"expected_revision,omitempty"`
	Set              ProjectSet      `json:"set,omitempty"`
	Clear            []string        `json:"clear,omitempty"`
	Add              ProjectMembers  `json:"add,omitempty"`
	Remove           ProjectMembers  `json:"remove,omitempty"`
}
type PrepareRequest struct {
	SchemaVersion int            `json:"schema_version"`
	Operation     string         `json:"operation"`
	Project       string         `json:"project,omitempty"`
	Items         []ProjectInput `json:"items"`
	Force         bool           `json:"force,omitempty"`
}
type Prepared struct {
	SchemaVersion  int             `json:"schema_version"`
	ServiceID      string          `json:"service_id"`
	ClientID       string          `json:"client_id"`
	Intent         Intent          `json:"intent"`
	RequiredClaims []RequiredClaim `json:"required_claims"`
	CanonicalJSON  string          `json:"canonical_json"`
	RequestHash    string          `json:"request_hash"`
}
type Project struct {
	SchemaVersion  int      `json:"schema_version"`
	ID             string   `json:"id"`
	Revision       int64    `json:"revision"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	RepositoryRefs []string `json:"repository_refs"`
	IssueIDs       []string `json:"issue_ids"`
}
type ChangedObject struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	BeforeRevision int64  `json:"before_revision"`
	Revision       int64  `json:"revision"`
}
type MutationResult struct {
	Outcome     string          `json:"outcome"`
	RequestHash string          `json:"request_hash"`
	Items       []ChangedObject `json:"results"`
}

// Canonical clones its input, normalizes only unordered sets, and retains text bytes.
func Canonical(in Intent) ([]byte, string, error) {
	b, e := json.Marshal(in)
	if e != nil {
		return nil, "", e
	}
	var v Intent
	if e = Decode(b, &v); e != nil {
		return nil, "", e
	}
	if v.SchemaVersion != 1 || v.Operation != "transaction" || !ValidUUID(v.ServiceID) || !ValidUUID(v.Actor.ClientID) || (v.Actor.Kind != "human" && v.Actor.Kind != "agent") || strings.TrimSpace(v.Actor.Name) == "" {
		return nil, "", fmt.Errorf("invalid intent identity")
	}
	if len(v.Operations) == 0 || len(v.Targets) == 0 {
		return nil, "", fmt.Errorf("empty transaction")
	}
	for _, refs := range [][]ObjectRef{v.Targets, v.Guards} {
		sort.Slice(refs, func(i, j int) bool { return refs[i].Type+refs[i].ID < refs[j].Type+refs[j].ID })
		for i, r := range refs {
			if !ResourceType(r.Type) || !ValidUUID(r.ID) || (r.ExpectedRevision != nil && *r.ExpectedRevision < 1) {
				return nil, "", fmt.Errorf("invalid object reference")
			}
			if i > 0 && r.Type == refs[i-1].Type && r.ID == refs[i-1].ID {
				return nil, "", fmt.Errorf("duplicate reference")
			}
		}
	}
	for _, r := range v.Guards {
		if r.ExpectedRevision == nil {
			return nil, "", fmt.Errorf("guard requires revision")
		}
	}
	for i := range v.Operations {
		o := &v.Operations[i]
		if !ResourceType(o.Type) || !ValidUUID(o.ID) || (o.Kind != "create" && o.Kind != "update") {
			return nil, "", fmt.Errorf("invalid operation")
		}
		if e := validateOperationFields(*o); e != nil {
			return nil, "", e
		}
		for _, p := range []*[]string{&o.Add.RepositoryRefs, &o.Remove.RepositoryRefs, o.Set.RepositoryRefs, &o.Add.Labels, &o.Remove.Labels, o.Set.Labels, &o.Add.Blocks, &o.Remove.Blocks, &o.Add.BlockedBy, &o.Remove.BlockedBy, &o.Add.Related, &o.Remove.Related} {
			if p == nil {
				continue
			}
			sort.Strings(*p)
			for j, s := range *p {
				if s == "" || j > 0 && s == (*p)[j-1] {
					return nil, "", fmt.Errorf("invalid or duplicate repository reference")
				}
			}
		}
		if o.Set.RepositoryRefs != nil && (len(o.Add.RepositoryRefs) > 0 || len(o.Remove.RepositoryRefs) > 0) {
			return nil, "", fmt.Errorf("replacement conflicts with member changes")
		}
		for _, s := range o.Add.RepositoryRefs {
			for _, r := range o.Remove.RepositoryRefs {
				if s == r {
					return nil, "", fmt.Errorf("contradictory repository changes")
				}
			}
		}
	}
	sort.Slice(v.Operations, func(i, j int) bool {
		a, b := v.Operations[i], v.Operations[j]
		return a.Type+a.ID+a.Kind < b.Type+b.ID+b.Kind
	})
	for i := 1; i < len(v.Operations); i++ {
		if v.Operations[i].Type == v.Operations[i-1].Type && v.Operations[i].ID == v.Operations[i-1].ID {
			return nil, "", fmt.Errorf("operations must be merged")
		}
	}
	b, e = json.Marshal(v)
	if e != nil {
		return nil, "", e
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:]), nil
}

func (p *ProjectSet) UnmarshalJSON(b []byte) error {
	type plain ProjectSet
	var fields map[string]json.RawMessage
	if e := Decode(b, &fields); e != nil {
		return e
	}
	if fields == nil {
		return fmt.Errorf("set must be an object")
	}
	for k, v := range fields {
		if string(v) == "null" && k != "parent_id" && k != "assignee" {
			return fmt.Errorf("set.%s cannot be null; use clear", k)
		}
	}
	var v plain
	if e := Decode(b, &v); e != nil {
		return e
	}
	*p = ProjectSet(v)
	if string(fields["parent_id"]) == "null" {
		var value *string
		p.ParentID = &value
	}
	if string(fields["assignee"]) == "null" {
		var value *string
		p.Assignee = &value
	}
	return nil
}
func (p *ProjectMembers) UnmarshalJSON(b []byte) error {
	type plain ProjectMembers
	var fields map[string]json.RawMessage
	if e := Decode(b, &fields); e != nil {
		return e
	}
	if fields == nil {
		return fmt.Errorf("member changes must be objects")
	}
	for k, v := range fields {
		if string(v) == "null" {
			return fmt.Errorf("%s cannot be null", k)
		}
	}
	var v plain
	if e := Decode(b, &v); e != nil {
		return e
	}
	*p = ProjectMembers(v)
	return nil
}
func (p *ProjectInput) UnmarshalJSON(b []byte) error {
	type plain ProjectInput
	var fields map[string]json.RawMessage
	if e := Decode(b, &fields); e != nil {
		return e
	}
	if fields == nil {
		return fmt.Errorf("item must be an object")
	}
	for k, v := range fields {
		if string(v) == "null" && k != "parent" && k != "assignee" {
			return fmt.Errorf("%s cannot be null", k)
		}
	}
	var v plain
	if e := Decode(b, &v); e != nil {
		return e
	}
	*p = ProjectInput(v)
	p.Fields = map[string]bool{}
	for k := range fields {
		p.Fields[k] = true
	}
	return nil
}
func (p ProjectInput) MarshalJSON() ([]byte, error) {
	type plain ProjectInput
	b, e := json.Marshal(plain(p))
	if e != nil {
		return nil, e
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(b, &fields)
	if emptySet(p.Set) && !p.Fields["set"] {
		delete(fields, "set")
	}
	if len(p.Add.RepositoryRefs) == 0 && len(p.Add.Labels) == 0 && len(p.Add.Blocks)+len(p.Add.BlockedBy)+len(p.Add.Related) == 0 && !p.Fields["add"] {
		delete(fields, "add")
	}
	if len(p.Remove.RepositoryRefs) == 0 && len(p.Remove.Labels) == 0 && len(p.Remove.Blocks)+len(p.Remove.BlockedBy)+len(p.Remove.Related) == 0 && !p.Fields["remove"] {
		delete(fields, "remove")
	}
	return json.Marshal(fields)
}
func (p ProjectInput) ValidateShape(kind string) error {
	for key := range p.Fields {
		allowed := false
		if kind == "create" {
			switch key {
			case "id", "title", "content", "repository_refs":
				allowed = true
			}
		} else {
			switch key {
			case "target", "expected_revision", "set", "clear", "add", "remove":
				allowed = true
			}
		}
		if !allowed {
			return fmt.Errorf("%s is inapplicable to project %s", key, kind)
		}
	}
	return nil
}
