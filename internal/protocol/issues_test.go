package protocol

import (
	"encoding/json"
	"testing"
)

func TestIssueCanonicalExtension(t *testing.T) {
	raw := `{"schema_version":1,"operation":"transaction","service_id":"00000000-0000-4000-8000-000000000001","actor":{"client_id":"00000000-0000-4000-8000-000000000002","name":"A","kind":"human"},"operations":[{"type":"issues","id":"00000000-0000-4000-8000-000000000003","kind":"create","set":{"title":"Straße","project_id":"00000000-0000-4000-8000-000000000004","body":"# bytes\r\n雪","labels":["z","a"]},"add":{},"remove":{}}],"targets":[{"type":"issues","id":"00000000-0000-4000-8000-000000000003","expected_revision":null}]}`
	var i Intent
	if e := Decode([]byte(raw), &i); e != nil {
		t.Fatal(e)
	}
	b, _, e := Canonical(i)
	if e != nil {
		t.Fatal(e)
	}
	var normalized Intent
	if e = Decode(b, &normalized); e != nil {
		t.Fatal(e)
	}
}

func TestNullableIssueIntentFields(t *testing.T) {
	var omitted, cleared ProjectSet
	if e := Decode([]byte(`{}`), &omitted); e != nil {
		t.Fatal(e)
	}
	if e := Decode([]byte(`{"assignee":null,"parent_id":null}`), &cleared); e != nil {
		t.Fatal(e)
	}
	if omitted.Assignee != nil || omitted.ParentID != nil || cleared.Assignee == nil || *cleared.Assignee != nil || cleared.ParentID == nil || *cleared.ParentID != nil {
		t.Fatal("lost omission/null distinction")
	}
	b, e := json.Marshal(cleared)
	if e != nil || string(b) != `{"assignee":null,"parent_id":null}` {
		t.Fatal(string(b), e)
	}
	for _, raw := range []string{`{"project_id":null}`, `{"issue_id":null}`, `{"body":null}`, `{"labels":null}`, `{"state":null}`, `{"author":null}`} {
		var v ProjectSet
		if e := Decode([]byte(raw), &v); e == nil {
			t.Fatal("accepted null", raw)
		}
	}
}
