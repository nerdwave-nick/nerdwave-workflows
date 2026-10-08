package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestQueryShorthandParsesLikeQueryFlag(t *testing.T) {
	for _, family := range []string{"projects", "issues", "milestones", "comments", "claims"} {
		want, err := Parse([]string{family, "list", "--query", "needle", "--limit", "5"})
		if family == "claims" {
			// claims list has no content filter; the shorthand must not invent one.
			if err == nil {
				t.Fatalf("claims list accepted --query")
			}
			if _, err := Parse([]string{family, "list", "-q", "needle"}); err == nil {
				t.Fatalf("claims list accepted -q")
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s --query: %v", family, err)
		}
		for _, spelling := range [][]string{{"-q", "needle"}, {"-q=needle"}, {"-qneedle"}, {"--query=needle"}} {
			got, err := Parse(append(append([]string{family, "list"}, spelling...), "--limit", "5"))
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("%s %v: %+v %v, want %+v", family, spelling, got, err, want)
			}
		}
	}
}

func TestQueryShorthandIsLiteralInValuePosition(t *testing.T) {
	got, err := Parse([]string{"issues", "list", "--title", "-q"})
	want, _ := Parse([]string{"issues", "list", "--title=-q"})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("--title -q: %+v %v, want %+v", got, err, want)
	}
	got, err = Parse([]string{"issues", "get", "--", "-q"})
	if err != nil || !reflect.DeepEqual(got.Positionals, []string{"-q"}) {
		t.Fatalf("operand after --: %+v %v", got, err)
	}
}

func TestOldQFlagIsRejected(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"issues", "list", "--session", "s", "--q", "needle"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "--q") {
		t.Fatalf("--q: code %d stderr %s", code, &errOut)
	}
}

func TestQueryFlagHelpAndCompletion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"issues", "list", "--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "-q, --query TEXT") {
		t.Fatalf("help: %d %s", code, &out)
	}
	offlineCompletion(t)
	if values, directive := completion(t, "issues", "list", "--query", ""); len(values) != 0 || directive != ":4" {
		t.Fatalf("--query value completion: %v %s", values, directive)
	}
	if values, directive := completion(t, "issues", "list", "-q", ""); len(values) != 0 || directive != ":4" {
		t.Fatalf("-q value completion: %v %s", values, directive)
	}
}
