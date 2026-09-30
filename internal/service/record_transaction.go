package service

import (
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// RecordState is one serialized snapshot, shared by transaction expansion and
// final-state validation. Future relationship/claim slices extend this boundary.
type RecordState map[string]any

func recordKey(typ, id string) string { return typ + "/" + id }
func (s *Server) ReadRecordState() (RecordState, error) {
	out := RecordState{}
	for _, typ := range []string{"projects", "issues", "comments"} {
		rs, e := s.Records(typ)
		if e != nil {
			return nil, e
		}
		for _, r := range rs {
			_, id, _, _, _ := recordIdentity(r)
			out[recordKey(typ, id)] = r
		}
	}
	return out, nil
}
func cloneState(in RecordState) RecordState {
	out := RecordState{}
	for k, v := range in {
		b := recordFields(v)
		raw := mustJSON(b)
		typ, _, _, _, _ := recordIdentity(v)
		out[k], _ = decodeRecord(typ, raw)
	}
	return out
}
func pointer(v string) *string    { return &v }
func nullable(v *string) **string { return &v }
func changeSet(current, add, remove []string) []string {
	out := union(current, add)
	for _, r := range remove {
		for n, v := range out {
			if v == r {
				out = append(out[:n], out[n+1:]...)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
func applyOperation(state RecordState, op protocol.Operation) error {
	key := recordKey(op.Type, op.ID)
	old, exists := state[key]
	if (op.Kind == "create") == exists {
		return protocol.E(409, "revision_conflict", "operation existence differs")
	}
	switch op.Type {
	case "projects":
		p := protocol.Project{SchemaVersion: 1, ID: op.ID, RepositoryRefs: []string{}, IssueIDs: []string{}}
		if exists {
			p = old.(protocol.Project)
		} else if op.Set.Title == nil {
			return invalid("creation needs title")
		}
		if op.Set.Title != nil {
			p.Title = *op.Set.Title
		}
		if op.Set.Description != nil {
			p.Description = *op.Set.Description
		}
		if op.Set.RepositoryRefs != nil {
			p.RepositoryRefs = append([]string{}, (*op.Set.RepositoryRefs)...)
		}
		p.RepositoryRefs = changeSet(p.RepositoryRefs, op.Add.RepositoryRefs, op.Remove.RepositoryRefs)
		state[key] = p
	case "issues":
		p := protocol.Issue{SchemaVersion: 1, ID: op.ID, State: "open", Labels: []string{}, ChildIDs: []string{}, CommentIDs: []string{}, Blocks: []string{}, BlockedBy: []string{}, Related: []string{}}
		if exists {
			p = old.(protocol.Issue)
		} else if op.Set.Title == nil || op.Set.ProjectID == nil {
			return invalid("issue creation needs title and project")
		}
		if op.Set.Title != nil {
			p.Title = *op.Set.Title
		}
		if op.Set.Body != nil {
			p.Body = *op.Set.Body
		}
		if op.Set.State != nil {
			p.State = *op.Set.State
		}
		if op.Set.Labels != nil {
			p.Labels = append([]string{}, (*op.Set.Labels)...)
		}
		p.Labels = changeSet(p.Labels, op.Add.Labels, op.Remove.Labels)
		p.Blocks = changeSet(p.Blocks, op.Add.Blocks, op.Remove.Blocks)
		p.BlockedBy = changeSet(p.BlockedBy, op.Add.BlockedBy, op.Remove.BlockedBy)
		p.Related = changeSet(p.Related, op.Add.Related, op.Remove.Related)
		if op.Set.Assignee != nil {
			p.Assignee = *op.Set.Assignee
		}
		if op.Set.ProjectID != nil {
			p.ProjectID = *op.Set.ProjectID
		}
		if op.Set.ParentID != nil {
			p.ParentID = *op.Set.ParentID
		}
		state[key] = p
	case "comments":
		p := protocol.Comment{SchemaVersion: 1, ID: op.ID}
		if exists {
			p = old.(protocol.Comment)
		} else if op.Set.IssueID == nil || op.Set.Author == nil {
			return invalid("comment creation needs issue and author")
		}
		if op.Set.Body != nil {
			p.Body = *op.Set.Body
		}
		if op.Set.Author != nil {
			p.Author = *op.Set.Author
		}
		if op.Set.IssueID != nil {
			p.IssueID = *op.Set.IssueID
		}
		state[key] = p
	default:
		return invalid("unsupported operation")
	}
	return nil
}
func (s *Server) finalState(before RecordState, ops []protocol.Operation) (RecordState, error) {
	after := cloneState(before)
	for _, op := range ops {
		if e := applyOperation(after, op); e != nil {
			return nil, e
		}
	}
	// Membership is derived solely from final ownership, never argument order.
	for k, r := range after {
		switch p := r.(type) {
		case protocol.Project:
			p.IssueIDs = []string{}
			after[k] = p
		case protocol.Issue:
			p.ChildIDs = []string{}
			p.CommentIDs = []string{}
			after[k] = p
		}
	}
	for _, k := range sortedKeys(after) {
		switch p := after[k].(type) {
		case protocol.Issue:
			projectKey := recordKey("projects", p.ProjectID)
			v, ok := after[projectKey]
			if !ok {
				return nil, invalid("issue project does not exist")
			}
			project := v.(protocol.Project)
			project.IssueIDs = append(project.IssueIDs, p.ID)
			after[projectKey] = project
			if p.ParentID != nil {
				parentKey := recordKey("issues", *p.ParentID)
				v, ok := after[parentKey]
				if !ok {
					return nil, invalid("parent does not exist")
				}
				parent := v.(protocol.Issue)
				if parent.ProjectID != p.ProjectID {
					return nil, invalid("parent must belong to the same project")
				}
				parent.ChildIDs = append(parent.ChildIDs, p.ID)
				after[parentKey] = parent
			}
		case protocol.Comment:
			issueKey := recordKey("issues", p.IssueID)
			v, ok := after[issueKey]
			if !ok {
				return nil, invalid("comment owner does not exist")
			}
			issue := v.(protocol.Issue)
			issue.CommentIDs = append(issue.CommentIDs, p.ID)
			after[issueKey] = issue
		}
	}
	for k, r := range after {
		switch p := r.(type) {
		case protocol.Project:
			sort.Strings(p.IssueIDs)
			after[k] = p
		case protocol.Issue:
			sort.Strings(p.ChildIDs)
			sort.Strings(p.CommentIDs)
			after[k] = p
		}
	}
	if e := s.validateRecordState(after); e != nil {
		return nil, e
	}
	return after, nil
}
func (s *Server) validateRecordState(state RecordState) error {
	if e := validateLinks(state); e != nil {
		return e
	}
	titles := map[string]bool{}
	for _, r := range state {
		switch p := r.(type) {
		case protocol.Project:
			if e := s.validateProject(p); e != nil {
				return e
			}
			key := "projects/" + strings.ToLower(p.Title)
			if titles[key] {
				return invalid("duplicate project title")
			}
			titles[key] = true
		case protocol.Issue:
			if e := validateIssueTitle(p.Title); e != nil {
				return e
			}
			if !utf8.ValidString(p.Body) || (p.State != "open" && p.State != "closed") || !sortedSet(p.Labels, false) || p.Assignee != nil && (strings.TrimSpace(*p.Assignee) == "" || !utf8.ValidString(*p.Assignee)) {
				return invalid("invalid issue fields")
			}
			key := p.ProjectID + "/" + IssueTitleKey(p.Title)
			if titles[key] {
				return invalid("duplicate issue title")
			}
			titles[key] = true
			for _, members := range [][]string{p.ChildIDs, p.CommentIDs, p.Blocks, p.BlockedBy, p.Related} {
				if !sortedSet(members, true) {
					return invalid("invalid issue relationships")
				}
			}
			seen := map[string]bool{p.ID: true}
			parent := p.ParentID
			for parent != nil {
				if seen[*parent] {
					return invalid("parent cycle")
				}
				seen[*parent] = true
				v, ok := state[recordKey("issues", *parent)]
				if !ok {
					return invalid("missing parent")
				}
				owner := v.(protocol.Issue)
				if owner.ProjectID != p.ProjectID {
					return invalid("parent project differs")
				}
				parent = owner.ParentID
			}
		case protocol.Comment:
			if strings.TrimSpace(p.Author) == "" || !utf8.ValidString(p.Author) || !utf8.ValidString(p.Body) {
				return invalid("invalid comment fields")
			}
		}
	}
	return nil
}
func stateTitleIndex(state RecordState) (TitleIndex, error) {
	ps := []protocol.Project{}
	for _, r := range state {
		if p, ok := r.(protocol.Project); ok {
			ps = append(ps, p)
		}
	}
	idx, e := titleIndex(ps)
	if e != nil {
		return idx, e
	}
	for _, r := range state {
		if p, ok := r.(protocol.Issue); ok {
			if idx.Issues[p.ProjectID] == nil {
				idx.Issues[p.ProjectID] = map[string]string{}
			}
			key := IssueTitleKey(p.Title)
			if _, exists := idx.Issues[p.ProjectID][key]; exists {
				return idx, invalid("duplicate issue title")
			}
			idx.Issues[p.ProjectID][key] = p.ID
		}
	}
	return idx, nil
}
func changedKeys(before, after RecordState) []string {
	out := []string{}
	for _, k := range sortedKeys(after) {
		if old, exists := before[k]; !exists || !same(old, after[k]) {
			out = append(out, k)
		}
	}
	return out
}
func recordRevision(r any) int64 {
	if r == nil {
		return 0
	}
	_, _, rev, _, _ := recordIdentity(r)
	return rev
}
func stampRecord(r any, stamp string) any {
	switch p := r.(type) {
	case protocol.Project:
		if p.Revision == 0 {
			p.CreatedAt = stamp
		}
		p.Revision++
		p.UpdatedAt = stamp
		return p
	case protocol.Issue:
		if p.Revision == 0 {
			p.CreatedAt = stamp
		}
		p.Revision++
		p.UpdatedAt = stamp
		return p
	case protocol.Comment:
		if p.Revision == 0 {
			p.CreatedAt = stamp
		}
		p.Revision++
		p.UpdatedAt = stamp
		return p
	}
	panic("unknown record")
}
func nextRecordTime(state RecordState) string {
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, r := range state {
		_, _, _, _, updated := recordIdentity(r)
		t, e := time.Parse(historyTimeFormat, updated)
		if e == nil && !now.After(t) {
			now = t.Add(time.Microsecond)
		}
	}
	return now.Format(historyTimeFormat)
}

// Structural reads are explicit guards unless the same owner is a target.
func requiredGuards(before, after RecordState, ops []protocol.Operation) map[string]bool {
	needed := map[string]bool{}
	var visitIssue func(RecordState, string, map[string]bool)
	visitIssue = func(state RecordState, id string, seen map[string]bool) {
		key := recordKey("issues", id)
		if seen[key] {
			return
		}
		seen[key] = true
		if _, exists := before[key]; exists {
			needed[key] = true
		}
		if v, ok := state[key]; ok {
			p := v.(protocol.Issue)
			needed[recordKey("projects", p.ProjectID)] = true
			if p.ParentID != nil {
				visitIssue(state, *p.ParentID, seen)
			}
			for _, next := range p.Blocks {
				visitIssue(state, next, seen)
			}
		}
	}
	for _, op := range ops {
		for _, state := range []RecordState{before, after} {
			switch op.Type {
			case "issues":
				visitIssue(state, op.ID, map[string]bool{})
			case "comments":
				if v, ok := state[recordKey(op.Type, op.ID)]; ok {
					visitIssue(state, v.(protocol.Comment).IssueID, map[string]bool{})
				}
			}
		}
	}
	return needed
}
func validateReferences(before, after RecordState, i protocol.Intent) error {
	required := map[string]bool{}
	for _, op := range i.Operations {
		required[recordKey(op.Type, op.ID)] = true
	}
	for _, k := range changedKeys(before, after) {
		required[k] = true
	}
	seen := map[string]bool{}
	for _, r := range i.Targets {
		k := recordKey(r.Type, r.ID)
		if !required[k] {
			return protocol.E(409, "affected_set_changed", "unexpected target")
		}
		seen[k] = true
	}
	if len(seen) != len(required) {
		return protocol.E(409, "affected_set_changed", "missing affected owner")
	}
	guards := map[string]bool{}
	for _, r := range i.Guards {
		k := recordKey(r.Type, r.ID)
		if seen[k] {
			return invalid("target also declared as guard")
		}
		guards[k] = true
	}
	for k := range requiredGuards(before, after, i.Operations) {
		if !seen[k] && !guards[k] {
			return protocol.E(409, "affected_set_changed", "missing structural guard")
		}
	}
	return checkRecordPreconditions(before, i)
}
func checkRecordPreconditions(before RecordState, i protocol.Intent) error {
	for _, r := range append(append([]protocol.ObjectRef{}, i.Targets...), i.Guards...) {
		k := recordKey(r.Type, r.ID)
		old, exists := before[k]
		if r.ExpectedRevision == nil {
			if exists {
				return protocol.E(409, "revision_conflict", "creation ID exists")
			}
		} else if !exists || recordRevision(old) != *r.ExpectedRevision {
			return protocol.E(409, "revision_conflict", "record revision changed")
		}
	}
	return nil
}
func (s *Server) recordTransaction(i protocol.Intent, hash string, c protocol.Client, tokens ...protocol.ClaimToken) ([]store.Write, protocol.MutationResult, error) {
	result := protocol.MutationResult{Outcome: "already_satisfied", RequestHash: hash, Items: []protocol.ChangedObject{}}
	if i.Actor.ClientID != c.ClientID {
		return nil, result, protocol.E(409, "wrong_client", "transaction client differs")
	}
	if i.ServiceID != s.Store.Identity.ServiceID {
		return nil, result, protocol.E(409, "wrong_service", "transaction service differs")
	}
	if i.Actor.Name != c.Actor.Name || i.Actor.Kind != c.Actor.Kind {
		return nil, result, protocol.E(409, "actor_changed", "actor differs")
	}
	if logicalMutationCount(i.Operations) > s.Config.Limits.ExplicitItems || len(i.Targets)+len(i.Guards) > s.Config.Limits.ExpandedRecords {
		return nil, result, protocol.E(413, "limit_exceeded", "transaction item limit exceeded")
	}
	before, e := s.ReadRecordState()
	if e != nil {
		return nil, result, e
	}
	if e = checkRecordPreconditions(before, i); e != nil {
		return nil, result, e
	}
	after, e := s.finalState(before, i.Operations)
	if e != nil {
		return nil, result, e
	}
	if e = validateReferences(before, after, i); e != nil {
		return nil, result, e
	}
	claimWrites, e := s.claimPolicy(before, after, i, c, tokens)
	if e != nil {
		return nil, result, e
	}
	changed := changedKeys(before, after)
	if len(changed) == 0 {
		return claimWrites, result, nil
	}
	if len(changed) > s.Config.Limits.ExpandedRecords {
		return nil, result, protocol.E(413, "limit_exceeded", "affected record limit exceeded")
	}
	stamp := nextRecordTime(before)
	for _, k := range changed {
		after[k] = stampRecord(after[k], stamp)
		typ, id, rev, _, _ := recordIdentity(after[k])
		result.Items = append(result.Items, protocol.ChangedObject{Type: typ, ID: id, BeforeRevision: rev - 1, Revision: rev})
	}
	result.Outcome = "applied"
	writes := claimWrites
	for _, k := range changed {
		r := after[k]
		_, id, rev, _, _ := recordIdentity(r)
		h := ProjectHistory{SchemaVersion: 1, OwnerID: id, BeforeRevision: rev - 1, Revision: rev, Timestamp: stamp, RequestHash: hash, Intent: i, Results: result.Items, Differences: recordDiff(before[k], r), BodyHunks: []BodyHunk{}}
		oldBody := ""
		if before[k] != nil {
			oldBody = recordBody(before[k])
		}
		if oldBody != recordBody(r) {
			h.BodyHunks = append(h.BodyHunks, BodyHunk{Before: oldBody, After: recordBody(r)})
		}
		writes = append(writes, recordWrites(r)...)
		writes = append(writes, store.JSONWrite(k+"/history/"+stamp+"-"+hash+".json", h))
	}
	idx, e := stateTitleIndex(after)
	if e != nil {
		return nil, result, e
	}
	writes = append(writes, store.JSONWrite("indexes/titles.json", idx))
	return writes, result, nil
}
func (s *Server) PrepareRecords(req protocol.PrepareRequest, c protocol.Client) (protocol.Prepared, error) {
	if strings.HasPrefix(req.Operation, "project.") {
		return s.PrepareProjects(req, c)
	}
	out := protocol.Prepared{}
	if req.Operation == "issue.link" || req.Operation == "issue.unlink" {
		return s.prepareLinks(req, c)
	}
	parts := strings.Split(req.Operation, ".")
	if len(parts) != 2 || (parts[0] != "issue" && parts[0] != "comment") || (parts[1] != "create" && parts[1] != "update" && parts[1] != "close" && parts[1] != "reopen") {
		return out, invalid("unsupported operation")
	}
	resource, kind := parts[0]+"s", parts[1]
	if resource == "comments" && kind != "create" && kind != "update" {
		return out, invalid("unsupported comment operation")
	}
	if req.SchemaVersion != 0 && req.SchemaVersion != 1 {
		return out, invalid("unsupported schema_version")
	}
	if len(req.Items) == 0 {
		return out, invalid("empty batch")
	}
	if len(req.Items) > s.Config.Limits.ExplicitItems {
		return out, protocol.E(413, "limit_exceeded", "too many items")
	}
	project := req.Project
	if project == "" && c.ProjectID != nil {
		project = *c.ProjectID
	}
	i := protocol.Intent{Force: req.Force, SchemaVersion: 1, Operation: "transaction", ServiceID: s.Store.Identity.ServiceID, Actor: protocol.DurableActor{ClientID: c.ClientID, Name: c.Actor.Name, Kind: c.Actor.Kind}}
	ops := map[string]protocol.Operation{}
	revisions := map[string]*int64{}
	for _, item := range req.Items {
		shapeKind := kind
		if kind == "close" || kind == "reopen" {
			shapeKind = "update"
		}
		if e := item.ValidateResourceShape(resource, kind); e != nil {
			return out, invalid(e.Error())
		}
		op := protocol.Operation{Type: resource, ID: item.ID, Kind: shapeKind, Set: item.Set, Add: item.Add, Remove: item.Remove}
		var rev *int64
		if kind == "create" {
			if !protocol.ValidUUID(op.ID) {
				return out, invalid("creation needs UUID")
			}
			op.Set.Body = item.Content
			if resource == "issues" {
				op.Set.Title = item.Title
				op.Set.State = item.State
				labels := append([]string{}, item.Labels...)
				op.Set.Labels = &labels
				op.Set.Assignee = nullable(item.Assignee)
				sel := item.Project
				if sel == "" {
					sel = project
				}
				p, e := s.ResolveProject(sel)
				if e != nil {
					return out, e
				}
				op.Set.ProjectID = &p.ID
				if item.Parent != nil {
					pid, e := s.resolveBatchParent(*item.Parent, p.ID, req.Items)
					if e != nil {
						return out, e
					}
					op.Set.ParentID = nullable(&pid)
				}
			} else {
				owner, e := s.ResolveIssue(item.Issue, project)
				if e != nil {
					return out, e
				}
				op.Set.IssueID = &owner.ID
				op.Set.Author = item.Author
				if op.Set.Author == nil {
					op.Set.Author = &c.Actor.Name
				}
			}
		} else {
			var old any
			var e error
			if resource == "issues" {
				old, e = s.ResolveIssue(item.Target, project)
			} else {
				old, e = s.ResolveComment(item.Target)
			}
			if e != nil {
				return out, e
			}
			_, op.ID, _, _, _ = recordIdentity(old)
			r := recordRevision(old)
			rev = &r
			if item.ExpectedRevision != nil {
				rev = item.ExpectedRevision
			}
			if item.Fields["parent"] && item.Parent == nil {
				if op.Set.ParentID != nil && *op.Set.ParentID != nil {
					return out, invalid("contradictory parent assignments")
				}
				op.Set.ParentID = nullable(nil)
			}
			if item.Parent != nil {
				owner := old.(protocol.Issue)
				pid, e := s.resolveBatchParent(*item.Parent, owner.ProjectID, nil)
				if e != nil {
					return out, e
				}
				if op.Set.ParentID != nil && (*op.Set.ParentID == nil || **op.Set.ParentID != pid) {
					return out, invalid("contradictory parent assignments")
				}
				op.Set.ParentID = nullable(&pid)
			}
			clears := map[string]bool{}
			for _, field := range item.Clear {
				if clears[field] {
					return out, invalid("duplicate clear field")
				}
				clears[field] = true
				switch field {
				case "content", "body":
					if op.Set.Body != nil && *op.Set.Body != "" {
						return out, invalid("clear conflicts with body")
					}
					op.Set.Body = pointer("")
				case "labels":
					if resource != "issues" || op.Set.Labels != nil && len(*op.Set.Labels) > 0 {
						return out, invalid("invalid clear labels")
					}
					v := []string{}
					op.Set.Labels = &v
				case "assignee":
					if resource != "issues" || op.Set.Assignee != nil && *op.Set.Assignee != nil {
						return out, invalid("invalid clear assignee")
					}
					op.Set.Assignee = nullable(nil)
				case "parent":
					if resource != "issues" || op.Set.ParentID != nil && *op.Set.ParentID != nil {
						return out, invalid("invalid clear parent")
					}
					op.Set.ParentID = nullable(nil)
				default:
					return out, invalid("unknown clear field")
				}
			}
			if kind == "close" || kind == "reopen" {
				state := "closed"
				if kind == "reopen" {
					state = "open"
				}
				if !same(op.Set, protocol.ProjectSet{}) || len(item.Clear) > 0 || len(op.Add.Labels)+len(op.Remove.Labels) > 0 {
					return out, invalid("state convenience command accepts only targets and revisions")
				}
				op.Set.State = &state
			}
		}
		check := i
		check.Operations = []protocol.Operation{op}
		check.Targets = []protocol.ObjectRef{{Type: resource, ID: op.ID, ExpectedRevision: rev}}
		b, _, e := protocol.Canonical(check)
		if e != nil {
			return out, invalid(e.Error())
		}
		var normalized protocol.Intent
		protocol.Decode(b, &normalized)
		op = normalized.Operations[0]
		if old, ok := ops[op.ID]; ok {
			if !same(revisions[op.ID], rev) {
				return out, invalid("incompatible revisions")
			}
			op, e = mergeRecordOperations(old, op)
			if e != nil {
				return out, e
			}
		}
		ops[op.ID] = op
		revisions[op.ID] = rev
	}
	for _, id := range sortedKeys(ops) {
		i.Operations = append(i.Operations, ops[id])
		i.Targets = append(i.Targets, protocol.ObjectRef{Type: resource, ID: id, ExpectedRevision: revisions[id]})
	}
	return s.prepareRecordIntent(i, c)
}
func (s *Server) prepareRecordIntent(i protocol.Intent, c protocol.Client) (protocol.Prepared, error) {
	out := protocol.Prepared{}
	before, e := s.ReadRecordState()
	if e != nil {
		return out, e
	}
	after, e := s.finalState(before, i.Operations)
	if e != nil {
		return out, e
	}
	present := map[string]bool{}
	for _, r := range i.Targets {
		present[recordKey(r.Type, r.ID)] = true
	}
	for _, k := range changedKeys(before, after) {
		if present[k] {
			continue
		}
		typ, id, _, _, _ := recordIdentity(after[k])
		v := recordRevision(before[k])
		i.Targets = append(i.Targets, protocol.ObjectRef{Type: typ, ID: id, ExpectedRevision: &v})
	}
	for _, r := range i.Targets {
		present[recordKey(r.Type, r.ID)] = true
	}
	for _, k := range sortedKeys(requiredGuards(before, after, i.Operations)) {
		if present[k] {
			continue
		}
		old, exists := before[k]
		if !exists {
			continue
		}
		typ, id, rev, _, _ := recordIdentity(old)
		i.Guards = append(i.Guards, protocol.ObjectRef{Type: typ, ID: id, ExpectedRevision: &rev})
	}
	b, hash, e := protocol.Canonical(i)
	if e != nil {
		return out, invalid(e.Error())
	}
	protocol.Decode(b, &i)
	if _, _, e = s.recordTransaction(i, hash, c); e != nil {
		return out, e
	}
	required, e := s.preparedClaims(before, after, i)
	if e != nil {
		return out, e
	}
	return protocol.Prepared{SchemaVersion: 1, ServiceID: s.Store.Identity.ServiceID, ClientID: c.ClientID, Intent: i, CanonicalJSON: string(b), RequestHash: hash, RequiredClaims: required}, nil
}
func (s *Server) resolveBatchParent(selector, project string, items []protocol.ProjectInput) (string, error) {
	if protocol.ValidUUID(selector) {
		for _, item := range items {
			if item.ID == selector {
				return selector, nil
			}
		}
	}
	p, e := s.ResolveIssue(selector, project)
	return p.ID, e
}
func mergeRecordOperations(a, b protocol.Operation) (protocol.Operation, error) {
	if a.Kind != b.Kind {
		return a, invalid("incompatible operations")
	}
	x, y := recordFields(a.Set), recordFields(b.Set)
	for k, v := range y {
		if old, exists := x[k]; exists && !same(old, v) {
			return a, invalid("contradictory scalar or set assignments")
		}
		x[k] = v
	}
	if e := protocol.Decode(mustJSON(x), &a.Set); e != nil {
		return a, e
	}
	a.Add.Blocks = union(a.Add.Blocks, b.Add.Blocks)
	a.Remove.Blocks = union(a.Remove.Blocks, b.Remove.Blocks)
	a.Add.BlockedBy = union(a.Add.BlockedBy, b.Add.BlockedBy)
	a.Remove.BlockedBy = union(a.Remove.BlockedBy, b.Remove.BlockedBy)
	a.Add.Related = union(a.Add.Related, b.Add.Related)
	a.Remove.Related = union(a.Remove.Related, b.Remove.Related)
	a.Add.Labels = union(a.Add.Labels, b.Add.Labels)
	a.Remove.Labels = union(a.Remove.Labels, b.Remove.Labels)
	return a, nil
}
