package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/term"
	"golang.org/x/text/width"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func terminalColumns(out io.Writer) int {
	if f, ok := out.(interface{ Fd() uintptr }); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	return 0
}

func (a *App) cliResult(r Result) ([]byte, error) { return a.cliResultWidth(r, terminalColumns(a.Out)) }

func (a *App) cliResultWidth(r Result, columns int) ([]byte, error) {
	var b bytes.Buffer
	if r.Outcome != "" {
		fmt.Fprintf(&b, "Outcome: %s\n", humanText(r.Outcome))
	}

	rows := make([]any, 0, len(r.Items))
	for _, item := range r.Items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var value any
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		rows = append(rows, value)
	}
	if len(rows) == 0 {
		name := a.Args.Command
		if a.Args.Verb == "history" {
			name = "history entries"
		} else if name == "transactions" {
			name = "retained requests"
		}
		if name == "" {
			name = "results"
		}
		fmt.Fprintf(&b, "No %s found.\n", humanText(name))
	} else if a.Args.Verb == "list" && a.humanTable(&b, rows, columns) {
		// The table is the complete requested page; details remain available via get.
	} else {
		for i, row := range rows {
			if i > 0 {
				fmt.Fprintln(&b)
			}
			humanDetail(&b, row, "")
		}
	}
	if r.RequestHash != "" {
		fmt.Fprintf(&b, "\nRequest hash: %s\n", humanText(r.RequestHash))
	}
	if r.NextCursor != nil {
		fmt.Fprintf(&b, "\nNext cursor: %s\nMore results available; repeat the same query with --cursor '%s'.\n", humanText(*r.NextCursor), humanText(strings.ReplaceAll(*r.NextCursor, "'", "'\\''")))
	}
	if r.ServerTime != "" {
		fmt.Fprintf(&b, "\nServer time: %s\n", humanText(r.ServerTime))
	}
	return b.Bytes(), nil
}

func tableColumns(command string) (keys, headers []string) {
	switch command {
	case "projects":
		keys = []string{"title", "issue_ids", "repository_refs", "id"}
		headers = []string{"TITLE", "ISSUES", "REPOSITORIES", "ID"}
	case "issues":
		keys = []string{"title", "state", "assignee", "labels", "id"}
		headers = []string{"TITLE", "STATE", "ASSIGNEE", "LABELS", "ID"}
	case "comments":
		keys = []string{"author", "body", "issue_id", "id"}
		headers = []string{"AUTHOR", "COMMENT", "ISSUE ID", "ID"}
	case "claims":
		keys = []string{"issue_id", "owner_client_id", "expires_at"}
		headers = []string{"ISSUE ID", "OWNER CLIENT ID", "EXPIRES AT"}
	}
	return
}

func (a *App) humanTable(b *bytes.Buffer, rows []any, columns int) bool {
	keys, headers := tableColumns(a.Args.Command)
	if len(keys) == 0 {
		return false
	}
	for _, row := range rows {
		if _, ok := row.(map[string]any); !ok {
			return false
		}
	}
	truncated := false
	var table bytes.Buffer
	tableRows := [][]string{headers}
	for _, row := range rows {
		obj := row.(map[string]any)
		cells := make([]string, len(keys))
		for i, key := range keys {
			value := obj[key]
			if key == "issue_ids" {
				if value == nil {
					value = json.Number("0")
				}
				if items, ok := value.([]any); ok {
					value = json.Number(strconv.Itoa(len(items)))
				}
			}
			cells[i] = strings.ReplaceAll(humanValue(value), "\n", ` \n `)
			if key != "id" && !strings.HasSuffix(key, "_id") {
				if displayColumns(cells[i]) > 60 {
					cells[i] = clipColumns(cells[i], 59) + "…"
					truncated = true
				}
			}
		}
		tableRows = append(tableRows, cells)
	}
	widths := make([]int, len(keys))
	for _, cells := range tableRows {
		for i, cell := range cells {
			if n := displayColumns(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for _, cells := range tableRows {
		for i, cell := range cells {
			table.WriteString(cell)
			if i < len(cells)-1 {
				table.WriteString(strings.Repeat(" ", widths[i]-displayColumns(cell)+2))
			}
		}
		table.WriteByte('\n')
	}
	if columns > 0 {
		for _, line := range strings.Split(table.String(), "\n") {
			if displayColumns(line) > columns {
				return false
			}
		}
	}
	b.Write(table.Bytes())
	name := a.Args.Command
	if len(rows) == 1 {
		name = strings.TrimSuffix(name, "s")
	}
	fmt.Fprintf(b, "\n%d %s\n", len(rows), name)
	if truncated {
		fmt.Fprintf(b, "Some text shortened; use lit %s get ID for full details (with the same --session).\n", a.Args.Command)
	}
	return true
}

func humanDetail(b *bytes.Buffer, value any, indent string) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			if key != "schema_version" {
				keys = append(keys, key)
			}
		}
		priority := map[string]int{"title": 1, "id": 2, "logical_session": 3, "client_id": 4, "status": 5, "state": 6, "revision": 7, "description": 8, "body": 9, "before": 10, "after": 11}
		sort.Slice(keys, func(i, j int) bool {
			pi, pj := priority[keys[i]], priority[keys[j]]
			if pi == 0 {
				pi = 100
			}
			if pj == 0 {
				pj = 100
			}
			if pi != pj {
				return pi < pj
			}
			return keys[i] < keys[j]
		})
		for _, key := range keys {
			item := v[key]
			switch nested := item.(type) {
			case map[string]any:
				fmt.Fprintf(b, "%s%s:\n", indent, humanLabel(key))
				humanDetail(b, nested, indent+"  ")
			case []any:
				complex := false
				for _, entry := range nested {
					switch entry.(type) {
					case map[string]any, []any:
						complex = true
					}
				}
				if complex {
					fmt.Fprintf(b, "%s%s:\n", indent, humanLabel(key))
					humanDetail(b, nested, indent+"  ")
				} else {
					humanField(b, indent, humanLabel(key), humanValue(item))
				}
			default:
				humanField(b, indent, humanLabel(key), humanValue(item))
			}
		}
	case []any:
		for i, item := range v {
			fmt.Fprintf(b, "%sItem %d:\n", indent, i+1)
			humanDetail(b, item, indent+"  ")
		}
	default:
		fmt.Fprintf(b, "%s%s\n", indent, humanValue(value))
	}
}
func humanField(b *bytes.Buffer, indent, label, value string) {
	if strings.Contains(value, "\n") {
		fmt.Fprintf(b, "%s%s:\n", indent, label)
		for _, line := range strings.Split(value, "\n") {
			fmt.Fprintf(b, "%s  %s\n", indent, line)
		}
	} else {
		fmt.Fprintf(b, "%s%s: %s\n", indent, label, value)
	}
}
func humanLabel(key string) string {
	words := strings.Split(key, "_")
	for i, word := range words {
		switch word {
		case "id":
			words[i] = "ID"
		case "ids":
			words[i] = "IDs"
		case "url":
			words[i] = "URL"
		default:
			if i == 0 && len(word) > 0 {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
	}
	return humanText(strings.Join(words, " "))
}
func humanValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "none"
	case string:
		if v == "" {
			return "—"
		}
		return humanText(v)
	case []any:
		if len(v) == 0 {
			return "none"
		}
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = humanValue(item)
		}
		return strings.Join(parts, ", ")
	default:
		return humanText(fmt.Sprint(value))
	}
}

// Preserve text and newlines, but show controls visibly so data cannot move the
// cursor, inject terminal escapes, or reverse the direction of neighboring text.
func humanText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029') {
			quoted := strconv.QuoteRune(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Count wide/fullwidth runes conservatively; combining marks consume no column.
func displayColumns(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		n++
		k := width.LookupRune(r).Kind()
		if k == width.EastAsianWide || k == width.EastAsianFullwidth {
			n++
		}
	}
	return n
}
func clipColumns(s string, max int) string {
	n := 0
	for i, r := range s {
		n += displayColumns(string(r))
		if n > max {
			return s[:i]
		}
	}
	return s
}
