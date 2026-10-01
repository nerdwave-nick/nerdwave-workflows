package cli

import (
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"net/url"
	"strings"
)

var commonQueryFlags = []string{"id", "q", "created-after", "created-before", "updated-after", "updated-before"}

func queryFlags(resource string) []string {
	out := append([]string{}, commonQueryFlags...)
	switch resource {
	case "projects":
		out = append(out, "title", "repository-ref")
	case "issues":
		out = append(out, "title", "parent", "state", "labels-all", "labels-any", "labels-none", "assignee", "blocked", "claimed", "owner-client-id", "milestone")
	case "milestones":
		out = append(out, "title")
	case "comments":
		out = append(out, "author")
	}
	return out
}
func querySet(k string) bool {
	return k == "id" || k == "repository-ref" || k == "labels-all" || k == "labels-any" || k == "labels-none"
}
func (a *App) queryValue(q map[string]any, k string, vs []string, project string) error {
	key := strings.ReplaceAll(k, "-", "_")
	if k == "milestone" {
		key = "milestone_id"
	}
	if querySet(k) {
		q[key] = vs
		return nil
	}
	switch k {
	case "blocked", "claimed":
		if vs[0] != "true" && vs[0] != "false" {
			return fmt.Errorf("invalid boolean filter")
		}
		q[key] = vs[0] == "true"
	case "parent":
		key = "parent_id"
		if vs[0] == "none" {
			q[key] = nil
		} else {
			var p protocol.Issue
			path := "/v1/issues/" + url.PathEscape(vs[0])
			if project != "" {
				path += "?project_id=" + url.QueryEscape(project)
			}
			if e := a.Call("GET", path, nil, nil, &p, false); e != nil {
				return e
			}
			q[key] = p.ID
		}
	case "assignee":
		if vs[0] == "none" {
			q[key] = nil
		} else {
			q[key] = vs[0]
		}
	default:
		q[key] = vs[0]
	}
	return nil
}
