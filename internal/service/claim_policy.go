package service

import (
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/store"
)

// Work requirements concern changed existing issues and explicit existing issue
// targets; pure structural guards grant no write permission. Appending comments
// is the sole membership-change exemption, even when another client owns work.
func claimRequirements(before, after RecordState, ops []protocol.Operation) []string {
	required := map[string]bool{}
	for _, op := range ops {
		if op.Type == "issues" && before[recordKey("issues", op.ID)] != nil {
			required[op.ID] = true
		}
		if op.Type == "comments" && op.Kind != "create" {
			if v := before[recordKey("comments", op.ID)]; v != nil {
				required[v.(protocol.Comment).IssueID] = true
			}
		}
	}
	for _, key := range changedKeys(before, after) {
		old, ok := before[key].(protocol.Issue)
		if !ok {
			continue
		}
		next := after[key].(protocol.Issue)
		// Ignore only the appended-comment membership itself; every other changed
		// field, including hierarchy and both link mirrors, needs ownership policy.
		old.CommentIDs = next.CommentIDs
		if !same(old, next) {
			required[old.ID] = true
		}
	}
	return sortedKeys(required)
}
func (s *Server) claimPolicy(before, after RecordState, i protocol.Intent, c protocol.Client, tokens []protocol.ClaimToken) ([]store.Write, error) {
	ids := claimRequirements(before, after, i.Operations)
	required := map[string]bool{}
	for _, id := range ids {
		required[id] = true
	}
	seen := map[string]bool{}
	for _, token := range tokens {
		if !protocol.ValidUUID(token.IssueID) || !protocol.ValidUUID(token.Token) || seen[token.IssueID] || !required[token.IssueID] {
			return nil, invalid("invalid or inapplicable claim token")
		}
		seen[token.IssueID] = true
		current, e := s.LiveClaim(token.IssueID)
		if e != nil {
			return nil, e
		}
		if current == nil || current.Token != token.Token || current.OwnerClientID != c.ClientID {
			return nil, protocol.E(409, "claim_conflict", "provided claim token is stale")
		}
	}
	writes := []store.Write{}
	for _, id := range ids {
		current, e := s.LiveClaim(id)
		if e != nil {
			return nil, e
		}
		if current != nil && current.OwnerClientID != c.ClientID && !i.Force {
			return nil, protocol.E(409, "claim_conflict", "another client owns affected issue")
		}
		closing := false
		for _, op := range i.Operations {
			if op.Type == "issues" && op.ID == id && op.Set.State != nil && *op.Set.State == "closed" {
				closing = true
			}
		}
		if current != nil && (i.Force || closing) {
			writes = append(writes, claimWrite(id, nil))
		}
	}
	return writes, nil
}
func (s *Server) preparedClaims(before, after RecordState, i protocol.Intent) ([]protocol.RequiredClaim, error) {
	out := []protocol.RequiredClaim{}
	for _, id := range claimRequirements(before, after, i.Operations) {
		c, e := s.LiveClaim(id)
		if e != nil {
			return nil, e
		}
		out = append(out, protocol.RequiredClaim{IssueID: id, Claim: c})
	}
	return out, nil
}

// Clear all owned records, including expired evidence, in the disconnect journal.
func (s *Server) disconnectWrites(c protocol.Client) ([]store.Write, error) {
	issues, e := s.Issues()
	if e != nil {
		return nil, e
	}
	writes := []store.Write{}
	for _, issue := range issues {
		claim, e := s.readClaim(issue.ID)
		if e != nil {
			return nil, e
		}
		if claim != nil && claim.OwnerClientID == c.ClientID {
			writes = append(writes, claimWrite(issue.ID, nil))
		}
	}
	writes = append(writes, store.JSONWrite("clients/"+c.ClientID+".json", c))
	return writes, nil
}
