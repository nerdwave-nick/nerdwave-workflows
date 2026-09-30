package cli

import (
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type claimView = protocol.RequiredClaim

func validateClaimArgs(a Args) error {
	switch a.Verb {
	case "acquire", "get", "renew", "release":
		if a.Has("all") {
			if len(a.Positionals) > 0 || a.Has("project") {
				return fmt.Errorf("--all cannot be combined with targets or --project")
			}
		} else if len(a.Positionals) == 0 {
			return fmt.Errorf("at least one issue target is required")
		}
	case "list":
		if len(a.Positionals) > 0 {
			return fmt.Errorf("claims list does not accept targets")
		}
	default:
		return fmt.Errorf("expected claims acquire, get, list, renew, or release")
	}
	if a.Verb == "list" {
		if a.Has("all") && (a.Has("limit") || a.Has("cursor")) {
			return fmt.Errorf("--all conflicts with pagination")
		}
		if a.Has("limit") {
			n, e := strconv.Atoi(a.One("limit"))
			if e != nil || n < 1 {
				return fmt.Errorf("invalid limit")
			}
		}
	}
	if a.Has("project") && a.One("project") == "" {
		return fmt.Errorf("--project cannot be empty")
	}
	if a.Has("for") && a.Has("until") {
		return fmt.Errorf("--for and --until are mutually exclusive")
	}
	if a.Has("for") {
		d, e := time.ParseDuration(a.One("for"))
		if e != nil || d <= 0 || d > time.Hour {
			return fmt.Errorf("--for must be a positive duration at most one hour")
		}
	}
	if a.Has("until") {
		if _, e := time.Parse(time.RFC3339Nano, a.One("until")); e != nil {
			return fmt.Errorf("--until must be RFC3339")
		}
	}
	return nil
}
func claimTarget(a Args, serverTime string) (string, error) {
	now, e := time.Parse(time.RFC3339Nano, serverTime)
	if e != nil {
		return "", protocol.E(500, "invalid_response", "claim snapshot omitted valid server time")
	}
	d := 30 * time.Minute
	if a.Has("for") {
		d, e = time.ParseDuration(a.One("for"))
		if e != nil {
			return "", e
		}
	}
	target := now.Add(d)
	if a.Has("until") {
		target, e = time.Parse(time.RFC3339Nano, a.One("until"))
		if e != nil {
			return "", e
		}
	}
	if !target.After(now) || target.After(now.Add(time.Hour)) {
		return "", protocol.E(400, "invalid_arguments", "claim expiry must be in the future and at most one hour from service time")
	}
	return target.UTC().Format(time.RFC3339Nano), nil
}
func (a *App) claims() (Result, error) {
	if a.Args.Verb == "list" {
		return a.claimList()
	}
	query := map[string]any{}
	if a.Args.Verb == "list" || a.Args.Has("all") {
		query["owner_client_id"] = a.Mapping.ClientID
	} else {
		query["targets"] = a.Args.Positionals
		if a.Args.Has("project") {
			query["project"] = a.Args.One("project")
		} else if a.Client.ProjectID != nil {
			query["project"] = *a.Client.ProjectID
		}
	}
	var snapshot struct {
		Items      []claimView `json:"items"`
		ServerTime string      `json:"server_time"`
	}
	if e := a.Call("POST", "/v1/claim-snapshots", query, nil, &snapshot, false); e != nil {
		return Result{}, e
	}
	if a.Args.Verb == "list" || a.Args.Verb == "get" {
		items := []any{}
		for _, v := range snapshot.Items {
			items = append(items, v)
		}
		return Result{Items: items}, nil
	}
	target := ""
	if a.Args.Verb == "acquire" || a.Args.Verb == "renew" {
		clock := snapshot.ServerTime
		if clock == "" {
			clock = a.ServerTime
		}
		var e error
		target, e = claimTarget(a.Args, clock)
		if e != nil {
			return Result{}, e
		}
	}
	items := []map[string]any{}
	for _, v := range snapshot.Items {
		if a.Args.Verb == "renew" && v.Claim == nil {
			return Result{}, protocol.E(409, "claim_expired", "cannot renew an absent or expired claim")
		}
		item := map[string]any{"issue_id": v.IssueID}
		if a.Args.Verb != "acquire" && v.Claim != nil {
			item["token"] = v.Claim.Token
		}
		if target != "" {
			item["extend_to"] = target
		}
		if a.Args.Has("force") {
			item["force"] = true
		}
		items = append(items, item)
	}
	selection := "explicit"
	if a.Args.Has("all") {
		selection = "all_owned"
	}
	payload := map[string]any{"schema_version": 1, "operation": "claims." + a.Args.Verb, "owner_client_id": a.Mapping.ClientID, "selection": selection, "items": items}
	var result struct {
		Outcome string      `json:"outcome"`
		Items   []claimView `json:"items"`
	}
	if e := a.Call("POST", "/v1/operations", payload, nil, &result, true); e != nil {
		return Result{}, e
	}
	out := []any{}
	for _, v := range result.Items {
		out = append(out, v)
	}
	return Result{Items: out, Outcome: result.Outcome}, nil
}

func preparedTokens(p protocol.Prepared) []protocol.ClaimToken {
	out := []protocol.ClaimToken{}
	for _, r := range p.RequiredClaims {
		if r.Claim != nil && r.Claim.OwnerClientID == p.ClientID {
			out = append(out, protocol.ClaimToken{IssueID: r.IssueID, Token: r.Claim.Token})
		}
	}
	return out
}

func (a *App) claimList() (Result, error) {
	q := map[string]any{}
	if a.Args.Has("file") {
		b, e := readUTF8(a.Args.One("file"))
		if e != nil {
			return Result{}, protocol.E(400, "invalid_arguments", e.Error())
		}
		if e = protocol.Decode(b, &q); e != nil || q == nil {
			return Result{}, protocol.E(400, "invalid_arguments", "invalid claim query object")
		}
		if typ, ok := q["type"]; ok {
			if typ != "claims" {
				return Result{}, protocol.E(400, "invalid_arguments", "query type differs")
			}
			delete(q, "type")
		}
	}
	for _, key := range []string{"owner-client-id", "issue-id", "sort", "direction", "limit", "cursor", "all"} {
		if a.Args.Has(key) {
			q[strings.ReplaceAll(key, "-", "_")] = a.Args.One(key)
		}
	}
	if _, exists := q["owner_client_id"]; !exists {
		q["owner_client_id"] = a.Mapping.ClientID
	}
	query := url.Values{}
	for key, value := range q {
		switch v := value.(type) {
		case string:
			query.Set(key, v)
		case float64:
			if key != "limit" || v != float64(int(v)) {
				return Result{}, protocol.E(400, "invalid_arguments", "invalid numeric query field")
			}
			query.Set(key, strconv.Itoa(int(v)))
		case bool:
			if key != "all" {
				return Result{}, protocol.E(400, "invalid_arguments", "invalid boolean query field")
			}
			query.Set(key, strconv.FormatBool(v))
		default:
			return Result{}, protocol.E(400, "invalid_arguments", "invalid claim query field")
		}
	}
	var result Result
	e := a.Call("GET", "/v1/claims?"+query.Encode(), nil, nil, &result, false)
	return result, e
}
