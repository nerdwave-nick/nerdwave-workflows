package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Markdown exports full single records as documents; other responses use
// escaped structured tables independent of terminal width.
func (a *App) markdownResult(r Result) ([]byte, error) {
	var b bytes.Buffer
	title := strings.TrimSpace(humanLabel(a.Args.Command) + " " + a.Args.Verb)
	if title == "" {
		title = "Results"
	}
	fmt.Fprintf(&b, "# %s\n\n", markdownText(title))
	if r.Outcome != "" {
		fmt.Fprintf(&b, "**Outcome:** %s\n\n", markdownText(r.Outcome))
	}
	rows := make([]any, 0, len(r.Items))
	for _, item := range r.Items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		var value any
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		rows = append(rows, value)
	}
	if document, ok, err := a.markdownDocument(rows); ok || err != nil {
		return document, err
	}
	keys, headers := tableColumns(a.Args.Command)
	table := a.Args.Verb == "list" && len(keys) > 0
	for _, row := range rows {
		if _, ok := row.(map[string]any); !ok {
			table = false
		}
	}
	if len(rows) == 0 {
		name := a.Args.Command
		if name == "" {
			name = "results"
		}
		if a.Args.Verb == "history" {
			name = "history entries"
		}
		fmt.Fprintf(&b, "No %s found.\n\n", markdownText(name))
	} else if table {
		markdownRow(&b, headers)
		separators := make([]string, len(keys))
		for i := range separators {
			separators[i] = "---"
		}
		markdownRow(&b, separators)
		for _, row := range rows {
			obj := row.(map[string]any)
			cells := make([]string, len(keys))
			for i, key := range keys {
				v := obj[key]
				if key == "issue_ids" {
					v = 0
					if ids, ok := obj[key].([]any); ok {
						v = len(ids)
					}
				}
				if key == "progress" {
					v = milestoneProgressSummary(v)
				}
				cells[i] = humanValue(v)
			}
			markdownRow(&b, cells)
		}
		fmt.Fprintf(&b, "\n%d %s\n\n", len(rows), markdownText(a.Args.Command))
	} else {
		for i, row := range rows {
			fmt.Fprintf(&b, "## Result %d\n\n| Field | Value |\n| --- | --- |\n", i+1)
			markdownFields(&b, row, "")
			fmt.Fprintln(&b)
		}
	}
	if r.RequestHash != "" {
		fmt.Fprintf(&b, "**Request hash:** %s\n\n", markdownText(r.RequestHash))
	}
	if r.NextCursor != nil {
		fmt.Fprintf(&b, "**Next cursor:** %s\n\nMore results available. Repeat the same query and filters with this cursor using `--cursor`.\n\n", markdownText(*r.NextCursor))
	}
	if r.ServerTime != "" {
		fmt.Fprintf(&b, "**Server time:** %s\n", markdownText(r.ServerTime))
	}
	return b.Bytes(), nil
}
func markdownRow(b *bytes.Buffer, cells []string) {
	fmt.Fprint(b, "|")
	for _, cell := range cells {
		fmt.Fprintf(b, " %s |", markdownText(cell))
	}
	fmt.Fprintln(b)
}
func markdownFields(b *bytes.Buffer, value any, path string) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			if key != "schema_version" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			markdownRow(b, []string{path, "none"})
		}
		for _, key := range keys {
			p := humanLabel(key)
			if path != "" {
				p = path + " / " + p
			}
			markdownFields(b, v[key], p)
		}
	case []any:
		if len(v) == 0 {
			markdownRow(b, []string{path, "none"})
		}
		for i, item := range v {
			markdownFields(b, item, fmt.Sprintf("%s / Item %d", path, i+1))
		}
	default:
		if path == "" {
			path = "Value"
		}
		markdownRow(b, []string{path, humanValue(value)})
	}
}
func markdownText(s string) string {
	var b strings.Builder
	for _, r := range humanText(s) {
		switch r {
		case '\n':
			b.WriteString("<br>")
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\\', '|', '`', '*', '_', '[', ']', '#', '!', '~':
			fmt.Fprintf(&b, "&#%d;", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
