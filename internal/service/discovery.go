package service

import (
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"strings"
	"time"
)

func validateDiscovery(q ProjectQuery) error {
	if q.State != "" && q.State != "open" && q.State != "closed" {
		return invalid("invalid issue state")
	}
	for _, id := range q.ID {
		if !protocol.ValidUUID(id) {
			return invalid("id filter requires UUID")
		}
	}
	for _, stamp := range []string{q.CreatedAfter, q.CreatedBefore, q.UpdatedAfter, q.UpdatedBefore} {
		if stamp != "" {
			if _, e := time.Parse(time.RFC3339Nano, stamp); e != nil {
				return invalid("invalid timestamp filter")
			}
		}
	}
	if q.ParentID != nil && *q.ParentID != nil && !protocol.ValidUUID(**q.ParentID) {
		return invalid("parent_id requires UUID or null")
	}
	if q.OwnerClientID != "" && !protocol.ValidUUID(q.OwnerClientID) {
		return invalid("owner_client_id requires UUID")
	}
	return nil
}
func (s *Server) matchesQuery(q ProjectQuery, r any, state RecordState) (bool, error) {
	_, id, _, created, updated := recordIdentity(r)
	if len(q.ID) > 0 && !contains(q.ID, id) {
		return false, nil
	}
	for _, b := range []struct {
		value, bound string
		after        bool
	}{{created, q.CreatedAfter, true}, {created, q.CreatedBefore, false}, {updated, q.UpdatedAfter, true}, {updated, q.UpdatedBefore, false}} {
		if b.bound != "" {
			v, _ := time.Parse(time.RFC3339Nano, b.value)
			limit, _ := time.Parse(time.RFC3339Nano, b.bound)
			if b.after && v.Before(limit) || !b.after && !v.Before(limit) {
				return false, nil
			}
		}
	}
	title, body := "", ""
	switch p := r.(type) {
	case protocol.Project:
		title = p.Title
		body = p.Description
		for _, ref := range q.RepositoryRef {
			if !contains(p.RepositoryRefs, ref) {
				return false, nil
			}
		}
	case protocol.Issue:
		if q.Claimed != nil || q.OwnerClientID != "" {
			claim, e := s.LiveClaim(p.ID)
			if e != nil {
				return false, e
			}
			if q.Claimed != nil && *q.Claimed != (claim != nil) {
				return false, nil
			}
			if q.OwnerClientID != "" && (claim == nil || claim.OwnerClientID != q.OwnerClientID) {
				return false, nil
			}
		}
		title = p.Title
		body = p.Body
		if q.State != "" && p.State != q.State {
			return false, nil
		}
		if q.ParentID != nil && !same(p.ParentID, *q.ParentID) || q.Assignee != nil && !same(p.Assignee, *q.Assignee) {
			return false, nil
		}
		for _, label := range q.LabelsAll {
			if !contains(p.Labels, label) {
				return false, nil
			}
		}
		if len(q.LabelsAny) > 0 {
			found := false
			for _, label := range q.LabelsAny {
				found = found || contains(p.Labels, label)
			}
			if !found {
				return false, nil
			}
		}
		for _, label := range q.LabelsNone {
			if contains(p.Labels, label) {
				return false, nil
			}
		}
		if q.Blocked != nil {
			blocked := false
			for _, id := range p.BlockedBy {
				blocker, ok := state[recordKey("issues", id)].(protocol.Issue)
				if !ok {
					return false, invalid("missing blocker")
				}
				blocked = blocked || blocker.State == "open"
			}
			if blocked != *q.Blocked {
				return false, nil
			}
		}
	case protocol.Comment:
		body = p.Body
		if q.Author != "" && p.Author != q.Author {
			return false, nil
		}
	}
	if q.Title != "" && IssueTitleKey(q.Title) != IssueTitleKey(title) {
		return false, nil
	}
	if q.Q != "" && !strings.Contains(IssueTitleKey(title+"\n"+body), IssueTitleKey(q.Q)) {
		return false, nil
	}
	return true, nil
}
