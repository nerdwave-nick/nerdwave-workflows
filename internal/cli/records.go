package cli

import (
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func parseRecordArgs(argv []string, a Args) (Args, error) {
	seenCommand, seenVerb := false, false
	group := map[string][]string{}
	a.Groups = append(a.Groups, group)
	global := map[string]bool{"session": true, "endpoint": true, "format": true, "timeout": true, "project": true}
	boundary := "issue"
	if a.Command == "milestones" {
		boundary = "milestone"
	}
	if a.Command == "comments" && a.Verb != "create" {
		boundary = "comment"
	}
	allowed := map[string]bool{}
	fields := []string{}
	switch a.Verb {
	case "create":
		fields = []string{boundary, "content", "content-file", "file"}
		if a.Command == "issues" {
			fields = append(fields, "parent", "state", "label", "assignee")
		} else if a.Command == "milestones" {
			fields = append(fields, "issue")
		} else {
			fields = append(fields, "author")
		}
	case "update":
		fields = []string{"content", "content-file", "clear", "revision", "file"}
		if a.Command == "issues" {
			fields = append(fields, boundary)
			fields = append(fields, "title", "parent", "state", "assignee", "add-label", "remove-label")
		} else if a.Command == "milestones" {
			fields = append(fields, "title", "add-issue", "remove-issue")
		} else {
			fields = append(fields, boundary)
			fields = append(fields, "author")
		}
	case "close", "reopen":
		if a.Command != "issues" {
			return a, fmt.Errorf("unknown comment command")
		}
		fields = []string{boundary, "revision", "file"}
	case "link", "unlink":
		if a.Command != "issues" {
			return a, fmt.Errorf("unsupported relation command")
		}
		fields = []string{"from", "to", "relation", "file"}
		boundary = "from"
	case "get":
	case "list":
		fields = append([]string{"sort", "direction", "limit", "cursor", "file"}, queryFlags(a.Command)...)
		allowed["all"] = false
		if a.Command == "issues" {
			allowed["all-projects"] = false
		} else if a.Command == "comments" {
			fields = append(fields, "issue")
		}
	case "history":
		fields = []string{"request-hash", "limit", "cursor"}
		allowed["all"] = false
	default:
		return a, fmt.Errorf("unknown resource command")
	}
	if a.Command != "milestones" && (a.Verb == "create" || a.Verb == "update" || a.Verb == "close" || a.Verb == "reopen" || a.Verb == "link" || a.Verb == "unlink") {
		allowed["force"] = false
	}
	for _, k := range fields {
		allowed[k] = true
	}
	for i := 0; i < len(argv); i++ {
		v := argv[i]
		if v == "--" {
			a.Positionals = append(a.Positionals, argv[i+1:]...)
			break
		}
		if !strings.HasPrefix(v, "--") {
			if !seenCommand && v == a.Command {
				seenCommand = true
				continue
			}
			if !seenVerb && v == a.Verb {
				seenVerb = true
				continue
			}
			a.Positionals = append(a.Positionals, v)
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(v, "--"), "=", 2)
		k := parts[0]
		valueNeeded, ok := allowed[k]
		if global[k] {
			valueNeeded = true
			ok = true
		}
		if !ok {
			if a.Command == "comments" && a.Verb == "create" && k == "comment" {
				return a, fmt.Errorf("comments create: use --issue ISSUE_REF instead of --comment")
			}
			return a, fmt.Errorf("unknown flag --%s", k)
		}
		value := "true"
		if valueNeeded {
			if len(parts) == 2 {
				value = parts[1]
			} else {
				i++
				if i >= len(argv) || strings.HasPrefix(argv[i], "--") {
					return a, fmt.Errorf("missing value for --%s", k)
				}
				value = argv[i]
			}
		} else if len(parts) > 1 {
			return a, fmt.Errorf("--%s takes no value", k)
		}
		dst := a.Values
		if !global[k] && k != "file" && k != "force" {
			if k == boundary && (a.Verb == "create" || a.Verb == "update" || a.Verb == "close" || a.Verb == "reopen" || a.Verb == "link" || a.Verb == "unlink") {
				group = map[string][]string{}
				a.Groups = append(a.Groups, group)
			}
			dst = group
		}
		repeat := querySet(k) || k == "to" || k == "label" || k == "add-label" || k == "remove-label" || k == "clear" || a.Command == "milestones" && a.Verb == "create" && k == "issue" || k == "add-issue" || k == "remove-issue"
		if len(dst[k]) > 0 && !repeat {
			return a, fmt.Errorf("duplicate flag --%s", k)
		}
		dst[k] = append(dst[k], value)
	}
	if len(a.Groups[0]) == 0 {
		a.Groups = a.Groups[1:]
	}
	if a.Has("file") && (len(a.Groups) > 0 || len(a.Positionals) > 0) {
		return a, fmt.Errorf("file conflicts with explicit inputs")
	}
	if a.Has("format") && !validOutputFormat(a.One("format")) {
		return a, fmt.Errorf("invalid format")
	}
	if a.Verb == "create" && len(a.Positionals) > 0 {
		return a, fmt.Errorf("create uses a resource item boundary")
	}
	if a.Verb == "list" && len(a.Positionals) > 0 {
		return a, fmt.Errorf("list does not accept targets")
	}
	if a.Verb == "history" && len(a.Positionals) > 1 {
		return a, fmt.Errorf("history requires one owner")
	}
	stdinUses := 0
	if a.One("file") == "-" {
		stdinUses++
	}
	for _, g := range a.Groups {
		if len(g["content-file"]) > 0 && g["content-file"][0] == "-" {
			stdinUses++
		}
	}
	if stdinUses > 1 {
		return a, fmt.Errorf("stdin may be consumed only once")
	}
	if a.Has("timeout") {
		d, e := time.ParseDuration(a.One("timeout"))
		if e != nil || d <= 0 {
			return a, fmt.Errorf("timeout must be positive")
		}
	}
	return a, nil
}
func (a *App) recordInputs() ([]protocol.ProjectInput, error) {
	kind := a.Args.Verb
	resource := a.Args.Command
	items := []protocol.ProjectInput{}
	if a.Args.Has("file") {
		b, e := readUTF8(a.Args.One("file"))
		if e != nil {
			return nil, e
		}
		if e = protocol.Decode(b, &items); e != nil {
			return nil, e
		}
		for n := range items {
			if e := items[n].ValidateResourceShape(resource, kind); e != nil {
				return nil, e
			}
			if kind == "create" {
				if items[n].ID != "" || items[n].Fields["id"] {
					return nil, fmt.Errorf("creation IDs are generated by the client")
				}
				items[n].ID = protocol.UUID()
			}
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("empty batch")
		}
		return items, nil
	}
	groups := a.Args.Groups
	if len(groups) == 0 && len(a.Args.Positionals) > 0 {
		groups = []map[string][]string{{}}
	}
	boundary := "issue"
	if resource == "milestones" {
		boundary = "milestone"
	}
	if resource == "comments" && kind != "create" {
		boundary = "comment"
	}
	for _, g := range groups {
		one := func(k string) string {
			if len(g[k]) > 0 {
				return g[k][0]
			}
			return ""
		}
		value := func(k string) *string {
			if _, ok := g[k]; ok {
				v := one(k)
				return &v
			}
			return nil
		}
		item := protocol.ProjectInput{}
		if kind == "link" || kind == "unlink" {
			if len(a.Args.Positionals) > 0 {
				return nil, fmt.Errorf("link uses --from and --to")
			}
			item.From = one("from")
			item.To = g["to"]
			item.Relation = one("relation")
			items = append(items, item)
			continue
		}
		content := value("content")
		if _, ok := g["content-file"]; ok {
			if content != nil {
				return nil, fmt.Errorf("content and content-file conflict")
			}
			b, e := readUTF8(one("content-file"))
			if e != nil {
				return nil, e
			}
			v := string(b)
			content = &v
		}
		if kind == "create" {
			if one(boundary) == "" {
				return nil, fmt.Errorf("each item requires --%s", boundary)
			}
			item.ID = protocol.UUID()
			item.Content = content
			if resource == "issues" {
				item.Title = value(boundary)
				item.Parent = value("parent")
				item.State = value("state")
				item.Labels = g["label"]
				item.Assignee = value("assignee")
			} else if resource == "milestones" {
				item.Title = value(boundary)
				ids, err := a.resolveIssueRefs(g["issue"], a.projectRef())
				if err != nil {
					return nil, err
				}
				item.IssueIDs = ids
			} else {
				item.Issue = one(boundary)
				item.Author = value("author")
			}
			items = append(items, item)
			continue
		}
		item.Set.Body = content
		item.Set.Title = value("title")
		item.Set.State = value("state")
		item.Set.Author = value("author")
		if v := value("assignee"); v != nil {
			item.Set.Assignee = &v
		}
		item.Parent = value("parent")
		item.Add.Labels = g["add-label"]
		item.Remove.Labels = g["remove-label"]
		item.Clear = g["clear"]
		if _, ok := g["revision"]; ok {
			n, e := strconv.ParseInt(one("revision"), 10, 64)
			if e != nil || n < 1 {
				return nil, fmt.Errorf("revision must be positive")
			}
			item.ExpectedRevision = &n
		}
		targets := a.Args.Positionals
		if one(boundary) != "" {
			if len(targets) > 0 {
				return nil, fmt.Errorf("grouped and positional targets conflict")
			}
			targets = []string{one(boundary)}
		} else if len(groups) != 1 {
			return nil, fmt.Errorf("each item requires --%s", boundary)
		}
		if len(targets) == 0 {
			if resource == "milestones" {
				return nil, fmt.Errorf("milestone targets required")
			}
			return nil, fmt.Errorf("issue/comment targets required")
		}
		for _, target := range targets {
			x := item
			x.Target = target
			if resource == "milestones" && (len(g["add-issue"]) > 0 || len(g["remove-issue"]) > 0) {
				project, err := a.resolveMilestoneProject(target, a.projectRef())
				if err != nil {
					return nil, err
				}
				x.Add.IssueIDs, err = a.resolveIssueRefs(g["add-issue"], project)
				if err != nil {
					return nil, err
				}
				x.Remove.IssueIDs, err = a.resolveIssueRefs(g["remove-issue"], project)
				if err != nil {
					return nil, err
				}
			}
			items = append(items, x)
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty mutation batch")
	}
	return items, nil
}
func (a *App) records() (Result, error) {
	result := Result{}
	resource := a.Args.Command
	project := ""
	if a.Args.Has("project") {
		project = a.Args.One("project")
	} else if a.Client.ProjectID != nil {
		project = *a.Client.ProjectID
	}
	switch a.Args.Verb {
	case "create", "update", "close", "reopen", "link", "unlink":
		items, e := a.recordInputs()
		if e != nil {
			return result, protocol.E(400, "invalid_arguments", e.Error())
		}
		var plan protocol.Prepared
		req := protocol.PrepareRequest{SchemaVersion: 1, Operation: strings.TrimSuffix(resource, "s") + "." + a.Args.Verb, Project: project, Items: items, Force: a.Args.Has("force")}
		if e = a.Call("POST", "/v1/transaction-previews", req, nil, &plan, false); e != nil {
			return result, e
		}
		b, hash, e := protocol.Canonical(plan.Intent)
		if e != nil || hash != plan.RequestHash || plan.SchemaVersion != 1 || plan.ServiceID != a.Meta.ServiceID || plan.ClientID != a.Client.ClientID || string(b) != plan.CanonicalJSON {
			return result, protocol.E(500, "invalid_response", "invalid prepared transaction")
		}
		var mutation protocol.MutationResult
		if e = a.Call("POST", "/v1/transactions", protocol.DurableRequest{SchemaVersion: 1, Intent: plan.Intent, RequestHash: hash, Claims: preparedTokens(plan)}, nil, &mutation, true); e != nil {
			return result, e
		}
		result.Outcome = mutation.Outcome
		result.RequestHash = mutation.RequestHash
		for _, r := range mutation.Items {
			result.Items = append(result.Items, r)
		}
		return result, nil
	case "get":
		if len(a.Args.Positionals) == 0 {
			return result, protocol.E(400, "invalid_arguments", "targets required")
		}
		targets := []map[string]string{}
		for _, target := range a.Args.Positionals {
			targets = append(targets, map[string]string{"type": resource, "selector": target, "project": project})
		}
		e := a.Call("POST", "/v1/snapshots", map[string]any{"schema_version": 1, "targets": targets}, nil, &result, false)
		return result, e
	case "list":
		q := map[string]any{"type": resource}
		if a.Args.Has("file") {
			b, e := readUTF8(a.Args.One("file"))
			if e != nil {
				return result, protocol.E(400, "invalid_arguments", e.Error())
			}
			if e = protocol.Decode(b, &q); e != nil {
				return result, protocol.E(400, "invalid_arguments", e.Error())
			}
			if typ, ok := q["type"]; ok && typ != resource {
				return result, protocol.E(400, "invalid_arguments", "query type differs")
			}
			q["type"] = resource
		}
		for _, g := range a.Args.Groups {
			for k, vs := range g {
				switch k {
				case "all", "all-projects":
					q[strings.ReplaceAll(k, "-", "_")] = true
				case "limit":
					n, e := strconv.Atoi(vs[0])
					if e != nil {
						return result, protocol.E(400, "invalid_arguments", "invalid limit")
					}
					q[k] = n
				case "issue":
					var owner protocol.Issue
					path := "/v1/issues/" + url.PathEscape(vs[0])
					if project != "" {
						path += "?project_id=" + url.QueryEscape(project)
					}
					if e := a.Call("GET", path, nil, nil, &owner, false); e != nil {
						return result, e
					}
					q["issue_id"] = owner.ID
				default:
					if e := a.queryValue(q, k, vs, project); e != nil {
						return result, protocol.E(400, "invalid_arguments", e.Error())
					}
				}
			}
		}
		if resource == "issues" && a.Args.Has("project") {
			if q["all_projects"] == true {
				return result, protocol.E(400, "invalid_arguments", "all-projects conflicts with explicit project")
			}
			if scoped, exists := q["project_id"]; exists && scoped != project {
				selected, ok := scoped.(string)
				if !ok {
					return result, protocol.E(400, "invalid_arguments", "invalid project scope")
				}
				var explicit, queryProject protocol.Project
				if e := a.Call("GET", "/v1/projects/"+url.PathEscape(project), nil, nil, &explicit, false); e != nil {
					return result, e
				}
				if e := a.Call("GET", "/v1/projects/"+url.PathEscape(selected), nil, nil, &queryProject, false); e != nil {
					return result, e
				}
				if explicit.ID != queryProject.ID {
					return result, protocol.E(400, "invalid_arguments", "file scope conflicts with explicit project")
				}
			}
		}
		if resource == "issues" && q["all_projects"] != true {
			if _, ok := q["project_id"]; !ok {
				q["project_id"] = project
			}
		}
		if resource == "milestones" {
			raw, exists := q["project_id"]
			if exists {
				scoped, ok := raw.(string)
				if !ok || scoped == "" {
					return result, protocol.E(400, "invalid_arguments", "file project_id must be a nonempty string")
				}
				if a.Args.Has("project") && scoped != project {
					var explicit, queryProject protocol.Project
					if e := a.Call("GET", "/v1/projects/"+url.PathEscape(project), nil, nil, &explicit, false); e != nil {
						return result, e
					}
					if e := a.Call("GET", "/v1/projects/"+url.PathEscape(scoped), nil, nil, &queryProject, false); e != nil {
						return result, e
					}
					if explicit.ID != queryProject.ID {
						return result, protocol.E(400, "invalid_arguments", "file scope conflicts with explicit project")
					}
				}
			} else if project != "" {
				q["project_id"] = project
			}
		}
		e := a.Call("POST", "/v1/snapshots", map[string]any{"schema_version": 1, "query": q}, nil, &result, false)
		return result, e
	case "history":
		if len(a.Args.Positionals) != 1 {
			return result, protocol.E(400, "invalid_arguments", "history requires one owner")
		}
		path := "/v1/" + resource + "/" + url.PathEscape(a.Args.Positionals[0])
		if (resource == "issues" || resource == "milestones") && project != "" {
			path += "?project_id=" + url.QueryEscape(project)
		}
		var owner struct {
			ID string `json:"id"`
		}
		if e := a.Call("GET", path, nil, nil, &owner, false); e != nil {
			return result, e
		}
		path = "/v1/" + resource + "/" + owner.ID + "/history"
		q := url.Values{}
		hash := ""
		for _, g := range a.Args.Groups {
			for k, vs := range g {
				if k == "request-hash" {
					hash = vs[0]
				} else {
					q.Set(k, vs[0])
				}
			}
		}
		if hash != "" {
			if len(q) > 0 {
				return result, protocol.E(400, "invalid_arguments", "history detail conflicts with pagination")
			}
			var v any
			detailPath := path + "/" + url.PathEscape(hash)
			if resource == "milestones" && project != "" {
				detailPath += "?project_id=" + url.QueryEscape(project)
			}
			e := a.Call("GET", detailPath, nil, nil, &v, false)
			result.Items = []any{v}
			return result, e
		}
		if resource == "milestones" && project != "" {
			q.Set("project_id", project)
		}
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		e := a.Call("GET", path, nil, nil, &result, false)
		return result, e
	}
	return result, protocol.E(400, "invalid_arguments", "unknown resource command")
}
