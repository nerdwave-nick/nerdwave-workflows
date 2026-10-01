package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func (q *ProjectQuery) UnmarshalJSON(b []byte) error {
	type plain ProjectQuery
	var fields map[string]json.RawMessage
	if e := protocol.Decode(b, &fields); e != nil {
		return e
	}
	if fields == nil {
		return fmt.Errorf("query must be an object")
	}
	for k, v := range fields {
		if string(v) == "null" && k != "parent_id" && k != "assignee" {
			return fmt.Errorf("query.%s cannot be null", k)
		}
	}
	var v plain
	if e := protocol.Decode(b, &v); e != nil {
		return e
	}
	*q = ProjectQuery(v)
	if string(fields["parent_id"]) == "null" {
		q.ParentID = nullable(nil)
	}
	if string(fields["assignee"]) == "null" {
		q.Assignee = nullable(nil)
	}
	q.Fields = map[string]bool{}
	for k := range fields {
		q.Fields[k] = true
	}
	return nil
}
func validateQueryFields(q ProjectQuery) error {
	allowed := map[string]bool{}
	for _, k := range []string{"type", "sort", "direction", "limit", "cursor", "all", "id", "q", "created_after", "created_before", "updated_after", "updated_before"} {
		allowed[k] = true
	}
	if q.Type != "comments" {
		allowed["title"] = true
	}
	switch q.Type {
	case "projects":
		allowed["repository_ref"] = true
	case "issues":
		for _, k := range []string{"project_id", "all_projects", "parent_id", "state", "labels_all", "labels_any", "labels_none", "assignee", "blocked", "claimed", "owner_client_id", "milestone_id"} {
			allowed[k] = true
		}
	case "milestones":
		allowed["project_id"] = true
	case "comments":
		allowed["issue_id"] = true
		allowed["author"] = true
	}
	b, _ := json.Marshal(q)
	var fields map[string]json.RawMessage
	json.Unmarshal(b, &fields)
	for k := range q.Fields {
		fields[k] = nil
	}
	for k := range fields {
		if !allowed[k] {
			return protocol.E(400, "unsupported_filter", "inapplicable query field: "+k)
		}
	}
	return nil
}

func (s *Server) resolveRecord(typ, selector, project string) (any, error) {
	switch typ {
	case "projects":
		return s.ResolveProject(selector)
	case "issues":
		return s.ResolveIssue(selector, project)
	case "comments":
		return s.ResolveComment(selector)
	case "milestones":
		return s.ResolveMilestone(selector, project)
	}
	return nil, invalid("unsupported resource")
}
func (s *Server) recordRoute(w http.ResponseWriter, r *http.Request, c protocol.Client) (bool, error) {
	typ := ""
	for _, t := range []string{"issues", "comments", "milestones"} {
		if r.URL.Path == "/v1/"+t || strings.HasPrefix(r.URL.Path, "/v1/"+t+"/") {
			typ = t
			break
		}
	}
	if typ == "" {
		return false, nil
	}
	project := r.URL.Query().Get("project_id")
	if project == "" && c.ProjectID != nil {
		project = *c.ProjectID
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/"+typ)
	if path == "" {
		switch r.Method {
		case "POST":
			return true, s.executeProjects(w, r, c, "", "create")
		case "GET":
			q, e := recordURLQuery(typ, r.URL.Query())
			if e != nil {
				return true, e
			}
			if q.ProjectID == "" && (typ == "issues" || typ == "milestones") && !q.AllProjects {
				q.ProjectID = project
			}
			page, e := s.recordPage(q)
			if e != nil {
				return true, e
			}
			return true, s.writeReadPage(w, page)
		}
		return true, protocol.E(404, "not_found", "unknown route")
	}
	selector := strings.TrimPrefix(path, "/")
	// History routes use UUID owners only, allowing slash-bearing literal titles.
	segments := strings.Split(selector, "/")
	if typ == "issues" && len(segments) == 2 && protocol.ValidUUID(segments[0]) && segments[1] == "relationship-changes" {
		if r.Method != "POST" {
			return true, protocol.E(404, "not_found", "unknown route")
		}
		return true, s.executeProjects(w, r, c, segments[0], "relationships")
	}
	if len(segments) >= 2 && protocol.ValidUUID(segments[0]) && (segments[1] == "history" || segments[1] == "requests") {
		if r.Method != "GET" || len(segments) > 3 {
			return true, protocol.E(404, "not_found", "unknown route")
		}
		owner, e := s.resolveRecord(typ, "id:"+segments[0], project)
		if e != nil {
			if pe, ok := e.(*protocol.Error); ok && pe.Code == "not_found" && segments[1] == "requests" && len(segments) == 3 {
				s.write(w, 200, map[string]any{"outcome": "not_recorded_here", "revision": nil})
				return true, nil
			}
			return true, e
		}
		hs, e := s.RecordHistory(typ, segments[0])
		if e != nil {
			return true, e
		}
		if len(segments) == 2 && segments[1] == "history" {
			values := r.URL.Query()
			// A milestone's project_id scopes the owner lookup above; it is not a
			// history pagination option. Strip it only after that scoped resolution.
			if typ == "milestones" {
				values.Del("project_id")
			}
			page, e := s.projectHistoryPage(typ+":"+segments[0], hs, values)
			if e != nil {
				return true, e
			}
			return true, s.writeReadPage(w, page)
		}
		if len(segments) != 3 {
			return true, invalid("request hash required")
		}
		for _, h := range hs {
			if h.RequestHash == segments[2] {
				if segments[1] == "requests" {
					s.write(w, 200, map[string]any{"outcome": "recorded", "request_hash": h.RequestHash, "results": h.Results})
				} else {
					s.write(w, 200, h)
				}
				return true, nil
			}
		}
		if segments[1] == "requests" {
			s.write(w, 200, map[string]any{"outcome": "not_recorded_here", "revision": recordRevision(owner)})
			return true, nil
		}
		return true, protocol.E(404, "not_found", "history entry not found")
	}
	owner, e := s.resolveRecord(typ, selector, project)
	if e != nil {
		return true, e
	}
	_, id, _, _, _ := recordIdentity(owner)
	switch r.Method {
	case "GET":
		projection, e := s.readProjection(owner)
		if e != nil {
			return true, e
		}
		s.write(w, 200, projection)
		return true, nil
	case "PATCH":
		return true, s.executeProjects(w, r, c, id, "update")
	}
	return true, protocol.E(404, "not_found", "unknown route")
}
func recordURLQuery(typ string, values url.Values) (ProjectQuery, error) {
	fields := map[string]any{"type": typ}
	for k, vs := range values {
		switch k {
		case "id", "repository_ref", "labels_all", "labels_any", "labels_none":
			fields[k] = vs
		default:
			if len(vs) != 1 {
				return ProjectQuery{}, invalid("duplicate query parameter")
			}
			v := vs[0]
			switch k {
			case "all", "all_projects", "blocked", "claimed":
				if v != "true" && v != "false" {
					return ProjectQuery{}, invalid("invalid boolean filter")
				}
				fields[k] = v == "true"
			case "limit":
				n, e := strconv.Atoi(v)
				if e != nil {
					return ProjectQuery{}, invalid("invalid limit")
				}
				fields[k] = n
			case "parent_id", "assignee":
				if v == "none" {
					fields[k] = nil
				} else {
					fields[k] = v
				}
			default:
				fields[k] = v
			}
		}
	}
	var q ProjectQuery
	if e := protocol.Decode(mustJSON(fields), &q); e != nil {
		return q, protocol.E(400, "unsupported_filter", e.Error())
	}
	return q, validateQueryFields(q)
}

// recordPage is the basic enumeration seam for dependency/discovery and grep.
// It materializes accepted records under Server.Mu; later filters extend Query.
func (s *Server) recordPage(q ProjectQuery) (ProjectPage, error) {
	if e := validateQueryFields(q); e != nil {
		return ProjectPage{}, e
	}
	if q.Type == "projects" {
		if q.ProjectID != "" || q.IssueID != "" || q.AllProjects {
			return ProjectPage{}, invalid("inapplicable project filters")
		}
	}
	out := ProjectPage{Items: []any{}}
	if q.Type != "issues" && q.Type != "comments" && q.Type != "projects" && q.Type != "milestones" {
		return out, invalid("invalid query type")
	}
	if q.Type == "issues" {
		if q.IssueID != "" || q.AllProjects && q.ProjectID != "" || (q.AllProjects || q.Fields["all_projects"]) && (q.MilestoneID != "" || q.Fields["milestone_id"]) {
			return out, invalid("inapplicable issue scope")
		}
		if !q.AllProjects {
			if q.ProjectID == "" {
				return out, invalid("select a project or all_projects")
			}
			p, e := s.ResolveProject(q.ProjectID)
			if e != nil {
				return out, e
			}
			q.ProjectID = p.ID
		}
		if q.MilestoneID != "" {
			if q.ProjectID == "" {
				return out, invalid("milestone filter requires project scope")
			}
			m, e := s.ResolveMilestone(q.MilestoneID, q.ProjectID)
			if e != nil {
				return out, e
			}
			q.MilestoneID = m.ID
		}
		if q.Fields["milestone_id"] && q.MilestoneID == "" {
			return out, invalid("milestone_id must not be empty")
		}
	} else if q.Type == "milestones" {
		if q.AllProjects || q.Fields["all_projects"] || q.IssueID != "" || q.MilestoneID != "" || q.ParentID != nil || q.State != "" || len(q.LabelsAll)+len(q.LabelsAny)+len(q.LabelsNone) > 0 || q.Assignee != nil || q.Blocked != nil || q.Claimed != nil || q.OwnerClientID != "" || q.Author != "" || len(q.RepositoryRef) > 0 {
			return out, invalid("inapplicable milestone filter")
		}
		if q.ProjectID == "" {
			return out, invalid("select a project for milestone query")
		}
		p, e := s.ResolveProject(q.ProjectID)
		if e != nil {
			return out, e
		}
		q.ProjectID = p.ID
	} else if q.Type == "comments" {
		if q.ProjectID != "" || q.AllProjects {
			return out, invalid("comments require issue scope")
		}
		if q.IssueID == "" {
			return out, invalid("comments require issue_id")
		}
		p, e := s.ResolveIssue(q.IssueID, "")
		if e != nil {
			return out, e
		}
		q.IssueID = p.ID
	}
	if q.All && (q.Limit != 0 || q.Cursor != "" || q.Fields["limit"] || q.Fields["cursor"]) {
		return out, invalid("all conflicts with pagination")
	}
	if q.Sort == "" {
		q.Sort = "created_at"
	}
	q.Sort = strings.ReplaceAll(q.Sort, "-", "_")
	if q.Sort != "created_at" && q.Sort != "updated_at" && (q.Sort != "title" || q.Type == "comments") {
		return out, invalid("invalid sort")
	}
	if q.Direction == "" {
		q.Direction = "asc"
	}
	if q.Direction != "asc" && q.Direction != "desc" {
		return out, invalid("invalid direction")
	}
	if q.Fields["limit"] && q.Limit == 0 {
		return out, invalid("limit must be positive")
	}
	if q.Limit == 0 {
		q.Limit = s.Config.Limits.DefaultPage
	}
	if q.Limit < 1 || q.Limit > s.Config.Limits.MaxPage {
		return out, invalid("invalid limit")
	}
	rs, e := s.Records(q.Type)
	if e != nil {
		return out, e
	}
	if e := validateDiscovery(q); e != nil {
		return out, e
	}
	state, e := s.ReadRecordState()
	if e != nil {
		return out, e
	}
	filtered := []any{}
	for _, r := range rs {
		switch p := r.(type) {
		case protocol.Issue:
			if !q.AllProjects && p.ProjectID != q.ProjectID {
				continue
			}
			if q.MilestoneID != "" {
				m, ok := state[recordKey("milestones", q.MilestoneID)].(protocol.Milestone)
				if !ok || !contains(m.IssueIDs, p.ID) {
					continue
				}
			}
		case protocol.Comment:
			if p.IssueID != q.IssueID {
				continue
			}
		case protocol.Milestone:
			if p.ProjectID != q.ProjectID {
				continue
			}
		}
		match, e := s.matchesQuery(q, r, state)
		if e != nil {
			return out, e
		}
		if !match {
			continue
		}
		filtered = append(filtered, r)
	}
	key := func(r any) string {
		_, _, _, created, updated := recordIdentity(r)
		if q.Sort == "updated_at" {
			return updated
		}
		if q.Sort == "title" {
			switch p := r.(type) {
			case protocol.Issue:
				return IssueTitleKey(p.Title)
			case protocol.Project:
				return strings.ToLower(p.Title)
			case protocol.Milestone:
				return IssueTitleKey(p.Title)
			}
		}
		return created
	}
	sort.Slice(filtered, func(i, j int) bool {
		a, b := key(filtered[i]), key(filtered[j])
		if a == b {
			_, a, _, _, _ = recordIdentity(filtered[i])
			_, b, _, _, _ = recordIdentity(filtered[j])
		}
		if q.Direction == "desc" {
			return a > b
		}
		return a < b
	})
	for _, set := range []*[]string{&q.ID, &q.RepositoryRef, &q.LabelsAll, &q.LabelsAny, &q.LabelsNone} {
		*set = union(nil, *set)
		sort.Strings(*set)
	}
	binding := q
	binding.Fields = nil
	binding.Cursor = ""
	hashBytes := sha256.Sum256(mustJSON(binding))
	hash := hex.EncodeToString(hashBytes[:])
	var cursor struct {
		Hash string `json:"hash"`
		Key  string `json:"key"`
		ID   string `json:"id"`
	}
	if q.Cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil || protocol.Decode(b, &cursor) != nil || cursor.Hash != hash || !protocol.ValidUUID(cursor.ID) || cursor.Key == "" {
			return out, protocol.E(400, "invalid_cursor", "cursor does not match query")
		}
	}
	for _, r := range filtered {
		_, id, _, _, _ := recordIdentity(r)
		k := key(r)
		if cursor.ID != "" && (q.Direction == "asc" && (k < cursor.Key || k == cursor.Key && id <= cursor.ID) || q.Direction == "desc" && (k > cursor.Key || k == cursor.Key && id >= cursor.ID)) {
			continue
		}
		if !q.All && len(out.Items) == q.Limit {
			last := out.Items[len(out.Items)-1]
			_, cursor.ID, _, _, _ = recordIdentity(last)
			cursor.Key = key(last)
			cursor.Hash = hash
			v := base64.RawURLEncoding.EncodeToString(mustJSON(cursor))
			out.NextCursor = &v
			break
		}
		out.Items = append(out.Items, r)
		if len(out.Items) > s.Config.Limits.ExpandedRecords {
			return out, protocol.E(413, "limit_exceeded", "snapshot item limit exceeded")
		}
	}
	return out, nil
}

func (p *ProjectSnapshot) UnmarshalJSON(b []byte) error {
	type plain ProjectSnapshot
	var fields map[string]json.RawMessage
	if e := protocol.Decode(b, &fields); e != nil {
		return e
	}
	if fields == nil {
		return fmt.Errorf("snapshot must be an object")
	}
	for k, v := range fields {
		if string(v) == "null" {
			return fmt.Errorf("snapshot.%s cannot be null", k)
		}
	}
	_, targets := fields["targets"]
	_, query := fields["query"]
	if targets == query {
		return fmt.Errorf("snapshot requires exactly targets or query")
	}
	var value plain
	if e := protocol.Decode(b, &value); e != nil {
		return e
	}
	*p = ProjectSnapshot(value)
	return nil
}
