package cli

import (
	"fmt"
	"strings"
	"time"
)

type Args struct {
	Command     string
	Verb        string
	Values      map[string][]string
	Positionals []string
	Groups      []map[string][]string
}

func (a Args) One(key string) string {
	v := a.Values[key]
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
func (a Args) Has(key string) bool { _, ok := a.Values[key]; return ok }

// Parse keeps global flags separate from resource-specific semantics. Resource
// slices extend allowedFlags; repeating item fields is handled by their parser.
func Parse(argv []string) (Args, error) {
	a, err := parseArgs(argv)
	if err == nil && a.Has("session") && a.One("session") == "" {
		return a, fmt.Errorf("--session must not be empty; omit the flag on connect to create a throwaway connection, or select via LIT_SESSION")
	}
	return a, err
}

func parseArgs(argv []string) (Args, error) {
	a := Args{Values: map[string][]string{}}
	// Locate grep before generic help/verb parsing so -- ends option parsing.
	for i := 0; i < len(argv); i++ {
		v := argv[i]
		if strings.HasPrefix(v, "--") {
			k := strings.SplitN(v, "=", 2)[0]
			if (globalValue(k) || k == "--project") && !strings.Contains(v, "=") {
				i++
			}
			continue
		}
		if v == "grep" {
			a.Command = "grep"
			return parseSearchArgs(argv, a)
		}
		break
	}
	for i := 0; i < len(argv); i++ {
		v := argv[i]
		if strings.HasPrefix(v, "--") {
			k := strings.SplitN(v, "=", 2)[0]
			if (globalValue(k) || k == "--project" && a.Command != "session") && !strings.Contains(v, "=") {
				i++
			}
			continue
		}
		if a.Command == "" {
			a.Command = v
			if v == "connect" || v == "disconnect" || v == "version" {
				break
			}
		} else {
			a.Verb = v
			break
		}
	}
	if a.Command == "" {
		return a, fmt.Errorf("a command is required")
	}
	if a.Command == "issues" || a.Command == "comments" {
		return parseRecordArgs(argv, a)
	}
	if a.Command == "projects" {
		return parseProjectArgs(argv, a)
	}
	allowed := allowedFlags(a)
	seenCommand, seenVerb := false, false
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
			if a.Verb != "" && !seenVerb && v == a.Verb {
				seenVerb = true
				continue
			}
			a.Positionals = append(a.Positionals, v)
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(v, "--"), "=", 2)
		k := parts[0]
		takesValue, ok := allowed[k]
		if !ok {
			return a, fmt.Errorf("unknown flag --%s", k)
		}
		if a.Has(k) {
			return a, fmt.Errorf("duplicate flag --%s", k)
		}
		value := "true"
		if takesValue {
			if len(parts) == 2 {
				value = parts[1]
			} else {
				i++
				if i == len(argv) || strings.HasPrefix(argv[i], "--") {
					return a, fmt.Errorf("missing value for --%s", k)
				}
				value = argv[i]
			}
		} else if len(parts) == 2 {
			return a, fmt.Errorf("--%s does not take a value", k)
		}
		a.Values[k] = []string{value}
	}
	if f := a.One("format"); a.Has("format") && !validOutputFormat(f) {
		return a, fmt.Errorf("--format must be cli, markdown or json")
	}
	if a.Has("timeout") {
		d, e := time.ParseDuration(a.One("timeout"))
		if e != nil || d <= 0 {
			return a, fmt.Errorf("--timeout must be a positive duration")
		}
	}
	return a, nil
}
func globalValue(k string) bool {
	return k == "--session" || k == "--endpoint" || k == "--format" || k == "--timeout"
}
func allowedFlags(a Args) map[string]bool {
	m := map[string]bool{"session": true, "endpoint": true, "format": true, "timeout": true}
	switch a.Command {
	case "transactions":
		if a.Verb == "status" {
			m["file"] = true
		}
	case "claims":
		switch a.Verb {
		case "list":
			for _, k := range []string{"owner-client-id", "issue-id", "sort", "direction", "limit", "cursor", "file"} {
				m[k] = true
			}
			m["all"] = false
		case "acquire":
			m["force"] = false
			m["for"] = true
			m["until"] = true
			m["project"] = true
		case "renew":
			m["all"] = false
			m["for"] = true
			m["until"] = true
			m["project"] = true
		case "release":
			m["all"] = false
			m["project"] = true
		case "get":
			m["project"] = true
		}
	case "connect":
		for _, k := range []string{"output-format", "client-id", "actor-name", "actor-kind", "project", "runtime-vendor", "runtime-session-id"} {
			m[k] = true
		}
	case "session":
		for _, k := range []string{"project", "output-format", "actor-name", "actor-kind", "runtime-vendor", "runtime-session-id"} {
			m[k] = a.Verb == "set"
		}
		if a.Verb == "get" {
			for _, k := range []string{"session-id", "service-id", "status", "revision"} {
				m[k] = false
			}
		}
	}
	return m
}
