package protocol

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCanonicalIntent(t *testing.T) {
	s := "feat/a"
	content := "# A\r\n<&> 雪\n"
	rev := int64(1)
	i := Intent{SchemaVersion: 1, Operation: "transaction", ServiceID: "00000000-0000-4000-8000-000000000001", Actor: DurableActor{ClientID: "00000000-0000-4000-8000-000000000002", Name: "A", Kind: "human"}, Targets: []ObjectRef{{Type: "projects", ID: "00000000-0000-4000-8000-000000000003", ExpectedRevision: &rev}}, Operations: []Operation{{Type: "projects", ID: "00000000-0000-4000-8000-000000000003", Kind: "update", Set: ProjectSet{Title: &s, Description: &content}, Add: ProjectMembers{RepositoryRefs: []string{"z", "a"}}}}}
	b, h, e := Canonical(i)
	if e != nil || len(h) != 64 {
		t.Fatal(e, h)
	}
	i.Operations[0].Add.RepositoryRefs = []string{"a", "z"}
	b2, h2, e := Canonical(i)
	if e != nil || !bytes.Equal(b, b2) || h != h2 {
		t.Fatal("unstable", e)
	}
	i.Operations[0].Add.RepositoryRefs = []string{"a", "a"}
	if _, _, e = Canonical(i); e == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestCanonicalPinnedFixtures(t *testing.T) {
	for _, name := range []string{"markdown", "clear", "omitted"} {
		t.Run(name, func(t *testing.T) {
			input, e := os.ReadFile("testdata/" + name + ".input.json")
			if e != nil {
				t.Fatal(e)
			}
			expected, e := os.ReadFile("testdata/" + name + ".canonical.json")
			if e != nil {
				t.Fatal(e)
			}
			expectedHash, e := os.ReadFile("testdata/" + name + ".sha256")
			if e != nil {
				t.Fatal(e)
			}
			var i Intent
			if e = Decode(input, &i); e != nil {
				t.Fatal(e)
			}
			actual, hash, e := Canonical(i)
			if e != nil || !bytes.Equal(actual, expected) || hash != strings.TrimSpace(string(expectedHash)) {
				t.Fatalf("fixture mismatch: %s %s %v", actual, hash, e)
			}
		})
	}
}
