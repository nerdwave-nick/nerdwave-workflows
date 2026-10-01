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

func TestCanonicalIssueRelationshipsKeepV1Hash(t *testing.T) {
	rev := int64(1)
	i := Intent{SchemaVersion: 1, Operation: "transaction", ServiceID: "11111111-1111-4111-8111-111111111111", Actor: DurableActor{ClientID: "22222222-2222-4222-8222-222222222222", Name: "fixture", Kind: "human"}, Operations: []Operation{{Type: "issues", ID: "33333333-3333-4333-8333-333333333333", Kind: "update", Add: ProjectMembers{Blocks: []string{"44444444-4444-4444-8444-444444444444"}, BlockedBy: []string{"55555555-5555-4555-8555-555555555555"}}}}, Targets: []ObjectRef{{Type: "issues", ID: "33333333-3333-4333-8333-333333333333", ExpectedRevision: &rev}}}
	_, hash, err := Canonical(i)
	if err != nil || hash != "82ce1de64b375ac8311c7fe6affccef4b61b4970bb8a5c2e79748ecf52caaa1c" {
		t.Fatalf("v1 canonical changed: hash=%s err=%v", hash, err)
	}
}

func TestMilestoneShapeRejectsPresentEmptyUnrelatedFields(t *testing.T) {
	var input ProjectInput
	if err := Decode([]byte(`{"id":"33333333-3333-4333-8333-333333333333","title":"M1","labels":[]}`), &input); err != nil {
		t.Fatal(err)
	}
	if err := input.ValidateResourceShape("milestones", "create"); err == nil {
		t.Fatal("accepted present empty issue-only field")
	}
	base := `{"schema_version":1,"operation":"transaction","service_id":"11111111-1111-4111-8111-111111111111","actor":{"client_id":"22222222-2222-4222-8222-222222222222","name":"fixture","kind":"human"},"operations":[{"type":"milestones","id":"33333333-3333-4333-8333-333333333333","kind":"create","set":{"title":"M1","project_id":"44444444-4444-4444-8444-444444444444"},"add":{"blocks":[]}}],"targets":[{"type":"milestones","id":"33333333-3333-4333-8333-333333333333","expected_revision":null}]}`
	var intent Intent
	if err := Decode([]byte(base), &intent); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Canonical(intent); err == nil {
		t.Fatal("accepted empty unrelated membership field")
	}
	intent.Operations[0].Add = ProjectMembers{}
	intent.Force = true
	if _, _, err := Canonical(intent); err == nil {
		t.Fatal("accepted milestone force")
	}
}
