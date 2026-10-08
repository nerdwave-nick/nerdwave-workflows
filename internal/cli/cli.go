// Package cli implements the public command boundary. Calls never start lit-server.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/clientendpoint"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

type App struct {
	Args           Args
	Out, Err       io.Writer
	StateDir       string
	Mapping        Mapping
	Meta           protocol.Meta
	Client         protocol.Client
	Endpoint       string
	Format         string
	HTTP           *http.Client
	Context        context.Context
	ServerTime     string
	LogicalSession string
}
type Result struct {
	Items       []any   `json:"items"`
	Outcome     string  `json:"outcome,omitempty"`
	RequestHash string  `json:"request_hash,omitempty"`
	NextCursor  *string `json:"next_cursor"`
	ServerTime  string  `json:"server_time,omitempty"`
}

func runTracker(argv []string, out, errOut io.Writer) int {
	a, e := Parse(argv)
	app := &App{Args: a, Out: out, Err: errOut, Format: a.One("format")}
	app.Format = offlineFormat(argv)
	if app.Format == "" {
		app.Format = "json"
	}
	if e != nil {
		return app.Error(protocol.E(400, "invalid_arguments", e.Error()))
	}
	if a.Command == "version" {
		if len(a.Positionals) > 0 {
			return app.Error(protocol.E(400, "invalid_arguments", "unexpected positional argument"))
		}
		fmt.Fprintln(out, protocol.ReleaseVersion)
		return 0
	}
	if e = validateArgs(a); e != nil {
		return app.Error(protocol.E(400, "invalid_arguments", e.Error()))
	}
	timeout := 30 * time.Second
	if a.Has("timeout") {
		timeout, _ = time.ParseDuration(a.One("timeout"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	app.Context = ctx
	app.HTTP = &http.Client{Timeout: timeout}
	app.StateDir, e = StateDir()
	if e != nil {
		return app.Error(protocol.E(500, "local_state_error", e.Error()))
	}
	if a.Command == "transactions" {
		return app.transactionStatus()
	}
	name := a.One("session")
	if name == "" {
		name = os.Getenv("LIT_SESSION")
	}
	if name == "" {
		if a.Command != "connect" {
			return app.Error(protocol.E(400, "missing_session", "select a logical session with --session NAME or LIT_SESSION; run connect to create a session"))
		}
		name = "throwaway-" + protocol.UUID()
	}
	app.LogicalSession = name
	path := mappingPath(app.StateDir, name) // Serialize a mapping across reads, registration, updates and disconnect.
	unlock, e := acquireLocal(path+".lock", time.Now().Add(timeout))
	if e != nil {
		return app.Error(protocol.E(409, "session_locked", e.Error()))
	}
	defer unlock()
	app.Mapping, e = readMapping(path)
	if e != nil && !os.IsNotExist(e) {
		return app.Error(protocol.E(500, "invalid_local_state", e.Error()))
	}
	exists := e == nil
	if !a.Has("format") {
		if f := cachedFormat(app.Mapping); f != "" {
			app.Format = f
		}
	}
	app.Endpoint, e = clientendpoint.Resolve(a.One("endpoint"), a.Has("endpoint"), os.Getenv("LIT_ENDPOINT"), app.Mapping.Endpoint)
	if e != nil {
		return app.Error(protocol.E(400, "invalid_endpoint", e.Error()))
	}
	var meta protocol.Meta
	if e = app.Call("GET", "/v1/meta", nil, nil, &meta, false); e != nil {
		return app.Error(e)
	}
	if meta.APIMajor != 1 || !protocol.ValidUUID(meta.ServiceID) {
		return app.Error(protocol.E(500, "incompatible_service", "unsupported API major or invalid service identity"))
	}
	app.Meta = meta
	if exists && app.Mapping.ServiceID != meta.ServiceID {
		return app.Error(protocol.E(409, "wrong_service", "endpoint is bound to a different service"))
	}
	if a.Command != "connect" && !exists {
		return app.Error(protocol.E(400, "not_connected", "connect this session explicitly first"))
	}
	if exists {
		if e = app.Call("GET", "/v1/clients/"+app.Mapping.ClientID, nil, nil, &app.Client, false); e != nil {
			return app.Error(e)
		}
		app.usePreference()
	}
	var result Result
	if a.Command == "connect" {
		result, e = app.connect(exists)
		if e == nil {
			app.Mapping = Mapping{1, app.Client.ClientID, meta.ServiceID, app.Endpoint}
			if x := WriteJSON(path, app.Mapping); x != nil {
				failure := protocol.E(500, "local_state_error", fmt.Sprintf("connected logical session %q (client %s), but could not save its mapping: %s; after repairing local state, recover with connect --session %q --client-id %s --endpoint %q", name, app.Client.ClientID, x, name, app.Client.ClientID, app.Endpoint))
				failure.Details = map[string]string{"logical_session": name, "client_id": app.Client.ClientID, "service_id": meta.ServiceID, "endpoint": app.Endpoint}
				e = failure
			}
		}
	} else {
		if app.Client.Status != "connected" && a.Command != "disconnect" {
			e = protocol.E(409, "client_disconnected", "connect explicitly to resume this client")
		} else {
			result, e = app.dispatch()
		}
	}
	if e != nil {
		return app.Error(e)
	}
	app.usePreference()
	if x := WriteJSON(cachePath(app.Mapping.ClientID), Cache{1, app.Mapping.ClientID, meta.ServiceID, app.preference()}); x != nil {
		fmt.Fprintln(errOut, "warning: operation succeeded; output cache could not be written:", x)
	}
	if a.Command == "connect" && a.Has("session-id-only") {
		if _, err := fmt.Fprintln(out, name); err != nil {
			return app.Error(err)
		}
		return 0
	}
	return app.Print(result)
}
func validateArgs(a Args) error {
	if a.Command == "claims" {
		return validateClaimArgs(a)
	}
	if a.Command == "grep" || a.Command == "projects" || a.Command == "issues" || a.Command == "comments" || a.Command == "milestones" {
		return nil
	}
	if len(a.Positionals) > 0 {
		return fmt.Errorf("unexpected positional argument")
	}
	switch a.Command {
	case "transactions":
		if a.Verb != "status" {
			return fmt.Errorf("expected transactions status")
		}
	case "connect", "disconnect":
	case "session":
		if a.Verb != "get" && a.Verb != "set" && a.Verb != "unset" {
			return fmt.Errorf("expected session get, set, or unset")
		}
		n := 0
		for k := range a.Values {
			if !globalValue("--" + k) {
				n++
				if a.Verb == "unset" && (k == "actor-name" || k == "actor-kind") {
					return fmt.Errorf("required actor fields cannot be unset")
				}
			}
		}
		if a.Verb != "get" && n == 0 {
			return fmt.Errorf("select at least one session property")
		}
	default:
		return fmt.Errorf("unknown command %s", a.Command)
	}
	if a.Has("actor-kind") && a.One("actor-kind") != "agent" && a.One("actor-kind") != "human" && a.Verb != "get" {
		return fmt.Errorf("actor-kind must be human or agent")
	}
	if a.Has("output-format") && (a.Verb == "set" || a.Command == "connect") && !validOutputFormat(a.One("output-format")) {
		return fmt.Errorf("output-format must be cli, markdown or json")
	}
	return nil
}
func (a *App) preference() string {
	if a.Client.OutputFormat != nil {
		return normalizeOutputFormat(*a.Client.OutputFormat)
	}
	return "cli"
}
func (a *App) usePreference() {
	if !a.Args.Has("format") {
		a.Format = a.preference()
	}
}
func (a *App) Call(method, path string, body any, headers map[string]string, dest any, mutation bool) (callErr error) {
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	req, e := http.NewRequestWithContext(a.Context, method, a.Endpoint+path, bytes.NewReader(b))
	if e != nil {
		return protocol.E(400, "invalid_endpoint", e.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	if a.Mapping.ClientID != "" {
		req.Header.Set("X-Lit-Client-ID", a.Mapping.ClientID)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	pending := ""
	var retained Pending
	var unlockPending func()
	defer func() {
		if unlockPending != nil {
			unlockPending()
		}
	}()
	defer func() {
		if pending == "" || callErr == nil {
			return
		}
		pe, ok := callErr.(*protocol.Error)
		if !ok || pe.Code != "outcome_uncertain" {
			return
		}
		pe.Details = map[string]any{"request_id": retained.RequestID}
		if a.Context.Err() != nil {
			return
		}
		status, e := a.reconcile(retained)
		if e != nil || status.Status == "uncertain" {
			return
		}
		if status.Status == "uncommitted" {
			callErr = protocol.E(503, "mutation_uncommitted", "read-only reconciliation found the request uncommitted at the observed snapshot; it was not retried")
			if e = os.Remove(pending); e != nil {
				fmt.Fprintln(a.Err, "warning: pending record cleanup failed:", e)
			}
			return
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(status.Response, &obj) != nil || !mutationResponseMatches(retained, status.Response) {
			return
		}
		if dest != nil && decodeResponseData(status.Response, dest) != nil {
			return
		}
		if e = os.Remove(pending); e != nil {
			fmt.Fprintln(a.Err, "warning: confirmed operation; pending record cleanup failed:", e)
		}
		callErr = nil
	}()
	if mutation {
		id := protocol.UUID()
		h := map[string]string{}
		for k, v := range req.Header {
			h[k] = v[0]
		}
		p := Pending{1, id, a.Meta.ServiceID, a.Mapping.ClientID, a.Endpoint, method, path, b, h, protocol.Now()}
		retained = p
		pending = filepath.Join(a.StateDir, "pending", id+".json")
		deadline := time.Now().Add(30 * time.Second)
		if d, ok := a.Context.Deadline(); ok {
			deadline = d
		}
		unlock, lockErr := acquireLocal(filepath.Join(a.StateDir, "locks", "pending", id+".lock"), deadline)
		if lockErr != nil {
			pending = ""
			return protocol.E(500, "pending_write_failed", "mutation was not sent: "+lockErr.Error())
		}
		unlockPending = unlock
		if e = WriteJSON(pending, p); e != nil {
			return protocol.E(500, "pending_write_failed", "mutation was not sent: "+e.Error())
		}
	}
	resp, e := a.HTTP.Do(req)
	if e != nil {
		if mutation {
			return protocol.E(0, "outcome_uncertain", "mutation outcome is uncertain; pending request retained")
		}
		return protocol.E(503, "service_unavailable", "cannot reach lit: "+e.Error())
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 32<<20+1))
	if e != nil || len(raw) > 32<<20 {
		if mutation {
			return protocol.E(0, "outcome_uncertain", "response incomplete; pending request retained")
		}
		return protocol.E(500, "invalid_response", "invalid service response")
	}
	var wire struct {
		Data       json.RawMessage `json:"data"`
		Items      json.RawMessage `json:"items"`
		NextCursor *string         `json:"next_cursor"`
		Error      *protocol.Error `json:"error"`
		ServerTime string          `json:"server_time"`
	}
	var checked map[string]json.RawMessage
	if e = protocol.Decode(raw, &checked); e == nil {
		e = decodeResponseData(raw, &wire)
	}
	if e != nil {
		if mutation {
			return protocol.E(0, "outcome_uncertain", "invalid mutation response; pending request retained")
		}
		return protocol.E(500, "invalid_response", "invalid JSON response")
	}
	if wire.Error != nil {
		if mutation && (!validErrorResponse(resp.StatusCode, wire.Error) || len(wire.Data) > 0 || wire.Items != nil) {
			return pendingError("invalid mutation error response")
		}
		wire.Error.Status = resp.StatusCode
		if mutation && resp.StatusCode >= 500 {
			return protocol.E(0, "outcome_uncertain", "service could not establish mutation outcome; pending request retained")
		}
		if pending != "" {
			_ = os.Remove(pending)
		}
		return wire.Error
	}
	if len(wire.Data) == 0 && wire.Items != nil {
		wire.Data, _ = json.Marshal(map[string]any{"items": wire.Items, "next_cursor": wire.NextCursor})
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || len(wire.Data) == 0 {
		if mutation {
			return protocol.E(0, "outcome_uncertain", "unexpected mutation response; pending request retained")
		}
		return protocol.E(500, "invalid_response", "unexpected service response")
	}
	if dest != nil {
		if e = decodeResponseData(wire.Data, dest); e != nil {
			if mutation {
				return protocol.E(0, "outcome_uncertain", "cannot decode mutation response; pending request retained")
			}
			return protocol.E(500, "invalid_response", e.Error())
		}
	}

	if mutation {
		var obj map[string]json.RawMessage
		if json.Unmarshal(wire.Data, &obj) != nil || !mutationResponseMatches(retained, wire.Data) {
			return protocol.E(0, "outcome_uncertain", "unrecognized mutation result; pending request retained")
		}
	}
	a.ServerTime = wire.ServerTime
	if pending != "" {
		if e = os.Remove(pending); e != nil {
			fmt.Fprintln(a.Err, "warning: confirmed operation; pending record cleanup failed:", e)
		}
	}
	return nil
}
func (a *App) Error(e error) int {
	pe, ok := e.(*protocol.Error)
	if !ok {
		pe = protocol.E(500, "failure", e.Error())
	}
	code := 1
	switch {
	case pe.Code == "outcome_uncertain":
		code = 4
	case pe.Code == "wrong_service" || pe.Code == "wrong_client" || pe.Code == "ambiguous_reference":
		code = 1
	case pe.Status == 409 || pe.Status == 423:
		code = 3
	case pe.Status == 400 || pe.Status == 413 || pe.Status == 422 || pe.Status == 428:
		code = 2
	}
	if a.Format == "cli" || a.Format == "human" {
		fmt.Fprintf(a.Err, "Error [%s]: %s\n", humanText(pe.Code), humanText(pe.Message))
		a.printErrorDetails(pe.Details, false)
	} else if a.Format == "markdown" {
		fmt.Fprintf(a.Err, "**Error [%s]:** %s\n", markdownText(pe.Code), markdownText(pe.Message))
		a.printErrorDetails(pe.Details, true)
	} else {
		_ = json.NewEncoder(a.Err).Encode(map[string]any{"error": pe})
	}
	return code
}
func (a *App) Print(r Result) int {
	if r.Items == nil {
		r.Items = []any{}
	}
	if r.ServerTime == "" {
		r.ServerTime = a.ServerTime
	}
	var b []byte
	var e error
	if a.Format == "json" {
		b, e = json.Marshal(r)
		b = append(b, '\n')
	} else if a.Format == "markdown" {
		b, e = a.markdownResult(r)
	} else if a.Args.Command == "grep" {
		b = []byte(humanText(string(a.humanSearch(r))))
	} else {
		b, e = a.cliResult(r)
	}
	if e != nil {
		return a.Error(e)
	}
	if _, e = a.Out.Write(b); e != nil {
		return a.Error(protocol.E(500, "output_failed", "operation completed but output failed: "+e.Error()))
	}
	return 0
}
func (a *App) connect(exists bool) (Result, error) {
	p := a.patch(false)
	id := a.Args.One("client-id")
	if exists && id != "" && id != a.Mapping.ClientID {
		return Result{}, protocol.E(409, "wrong_client", "local session already maps to a different client")
	}
	headers := map[string]string{}
	if exists {
		id = a.Mapping.ClientID
	}
	if id != "" {
		p["client_id"] = id
		if !exists {
			a.Mapping.ClientID = id
			var c protocol.Client
			if e := a.Call("GET", "/v1/clients/"+id, nil, nil, &c, false); e != nil {
				return Result{}, e
			}
			a.Client = c
		}
		headers["If-Match"] = fmt.Sprintf("\"client:%d\"", a.Client.StateRevision)
	} else {
		actor, ok := p["actor"].(map[string]any)
		if !ok {
			actor = map[string]any{}
		}
		if _, ok = actor["name"]; !ok {
			name := os.Getenv("USER")
			if u, e := user.Current(); e == nil {
				name = u.Username
			}
			if name == "" {
				name = "human"
			}
			actor["name"] = name
		}
		if _, ok = actor["kind"]; !ok {
			actor["kind"] = "human"
		}
		p["actor"] = actor
	}
	if a.Args.Has("format") && !a.Args.Has("output-format") {
		p["output_format"] = a.Args.One("format")
	}
	var wire struct {
		Client protocol.Client `json:"client"`
		Meta   protocol.Meta   `json:"meta"`
	}
	if e := a.Call("POST", "/v1/connect", p, headers, &wire, true); e != nil {
		return Result{}, e
	}
	a.Client = wire.Client
	return Result{Items: []any{a.clientItem(nil)}}, nil
}
func (a *App) patch(unset bool) map[string]any {
	p := map[string]any{}
	actor := map[string]any{}
	runtime := map[string]any{}
	for k := range a.Args.Values {
		var v any = a.Args.One(k)
		if unset {
			v = nil
		}
		switch k {
		case "actor-name":
			actor["name"] = v
		case "actor-kind":
			actor["kind"] = v
		case "project":
			p["project_id"] = v
		case "output-format":
			p["output_format"] = v
		case "runtime-vendor":
			runtime["vendor"] = v
		case "runtime-session-id":
			runtime["session_id"] = v
		}
	}
	if len(actor) > 0 {
		p["actor"] = actor
	}
	if len(runtime) > 0 {
		p["runtime"] = runtime
	}
	return p
}
func (a *App) dispatch() (Result, error) {
	h := map[string]string{"If-Match": fmt.Sprintf("\"client:%d\"", a.Client.StateRevision)}
	switch a.Args.Command {
	case "claims":
		return a.claims()
	case "grep":
		return a.grep()
	case "projects":
		return a.projects()
	case "issues", "comments", "milestones":
		return a.records()
	case "disconnect":
		var v struct {
			Client  protocol.Client `json:"client"`
			Outcome string          `json:"outcome"`
		}
		if e := a.Call("POST", "/v1/disconnect", map[string]any{}, h, &v, true); e != nil {
			return Result{}, e
		}
		a.Client = v.Client
		return Result{Items: []any{a.clientItem(nil)}, Outcome: v.Outcome}, nil
	case "session":
		if a.Args.Verb != "get" {
			if e := a.Call("PATCH", "/v1/clients/"+a.Client.ClientID, a.patch(a.Args.Verb == "unset"), h, &a.Client, true); e != nil {
				return Result{}, e
			}
		}
		var selected map[string][]string
		if a.Args.Verb == "get" {
			selected = a.Args.Values
		}
		return Result{Items: []any{a.clientItem(selected)}}, nil
	}
	return Result{}, protocol.E(400, "invalid_arguments", "unknown command")
}
func (a *App) clientItem(selection map[string][]string) map[string]any {
	c := a.Client
	all := map[string]any{"logical_session": a.LogicalSession, "schema_version": c.SchemaVersion, "client_id": c.ClientID, "service_id": a.Meta.ServiceID, "state_revision": c.StateRevision, "status": c.Status, "actor": c.Actor, "runtime": c.Runtime, "project_id": c.ProjectID, "output_format": c.OutputFormat}
	v := map[string]any{}
	for k := range selection {
		switch k {
		case "session-id":
			v["client_id"] = c.ClientID
		case "service-id":
			v["service_id"] = a.Meta.ServiceID
		case "status":
			v["status"] = c.Status
		case "revision":
			v["state_revision"] = c.StateRevision
		case "actor-name":
			v["actor_name"] = c.Actor.Name
		case "actor-kind":
			v["actor_kind"] = c.Actor.Kind
		case "runtime-vendor":
			v["runtime_vendor"] = c.Runtime.Vendor
		case "runtime-session-id":
			v["runtime_session_id"] = c.Runtime.SessionID
		case "project":
			v["project_id"] = c.ProjectID
		case "output-format":
			v["output_format"] = c.OutputFormat
		}
	}
	if len(v) == 0 {
		return all
	}
	return v
}

func validMutationResponse(path string, obj map[string]json.RawMessage) bool {
	if raw, ok := obj["outcome"]; ok {
		var outcome string
		if json.Unmarshal(raw, &outcome) != nil {
			return false
		}
		switch outcome {
		case "applied", "already_satisfied", "already_disconnected", "no_active_claim":
		default:
			return false
		}
	}
	if path == "/v1/connect" || path == "/v1/disconnect" || strings.HasPrefix(path, "/v1/clients/") {
		raw := obj["client"]
		if strings.HasPrefix(path, "/v1/clients/") {
			raw, _ = json.Marshal(obj)
		}
		var c protocol.Client
		if json.Unmarshal(raw, &c) != nil || c.SchemaVersion != 1 || !protocol.ValidUUID(c.ClientID) || c.StateRevision < 1 || (c.Status != "connected" && c.Status != "disconnected") {
			return false
		}
		return true
	}
	if path == "/v1/operations" {
		var outcome string
		var items []claimView
		if json.Unmarshal(obj["outcome"], &outcome) != nil || (outcome != "applied" && outcome != "already_satisfied" && outcome != "no_active_claim") || json.Unmarshal(obj["items"], &items) != nil || items == nil {
			return false
		}
		var rawItems []map[string]json.RawMessage
		if json.Unmarshal(obj["items"], &rawItems) != nil {
			return false
		}
		for _, raw := range rawItems {
			if _, ok := raw["claim"]; !ok {
				return false
			}
		}
		seen := map[string]bool{}
		for _, item := range items {
			if !protocol.ValidUUID(item.IssueID) || seen[item.IssueID] {
				return false
			}
			seen[item.IssueID] = true
			if c := item.Claim; c != nil {
				if c.SchemaVersion != 1 || c.IssueID != item.IssueID || !protocol.ValidUUID(c.OwnerClientID) || !protocol.ValidUUID(c.Token) {
					return false
				}
				start, e := time.Parse(time.RFC3339Nano, c.AcquiredAt)
				if e != nil {
					return false
				}
				end, e := time.Parse(time.RFC3339Nano, c.ExpiresAt)
				if e != nil || !end.After(start) {
					return false
				}
			}
		}
		return true
	}
	if path == "/v1/transactions" || strings.HasPrefix(path, "/v1/projects") {
		var outcome, hash string
		var results []protocol.ChangedObject
		if json.Unmarshal(obj["outcome"], &outcome) != nil || (outcome != "applied" && outcome != "already_satisfied") || json.Unmarshal(obj["request_hash"], &hash) != nil || len(hash) != 64 || json.Unmarshal(obj["results"], &results) != nil || results == nil {
			return false
		}
		return true
	}
	return len(obj) > 0
}

// Syntax failures also honor explicit output and this cwd's confirmed cache,
// without contacting a service or creating local state.
func offlineFormat(argv []string) string {
	name := os.Getenv("LIT_SESSION")
	format := ""
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--" {
			break
		}
		parts := strings.SplitN(argv[i], "=", 2)
		if parts[0] != "--session" && parts[0] != "--format" {
			continue
		}
		v := ""
		if len(parts) == 2 {
			v = parts[1]
		} else if i+1 < len(argv) {
			i++
			v = argv[i]
		}
		if parts[0] == "--session" {
			name = v
		} else {
			format = v
		}
	}
	if validOutputFormat(format) {
		return format
	}
	if name == "" {
		return ""
	}
	dir, e := StateDir()
	if e != nil {
		return ""
	}
	m, e := readMapping(mappingPath(dir, name))
	if e != nil {
		return ""
	}
	return cachedFormat(m)
}

// Response maps retain exact JSON numbers; typed protocol fields still decode
// into their declared integer types. Share this with reconciliation so recovered
// responses do not lose precision compared with direct responses.
func decodeResponseData(raw []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(dest)
}
