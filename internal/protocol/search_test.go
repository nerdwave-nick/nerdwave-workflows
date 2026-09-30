package protocol

import "testing"

func TestSearchStrictRequests(t *testing.T) {
	for _, s := range []string{`{"query":"x","Cursor":""}`, `{"query":"x","Scope":{"project_id":null}}`, `{"query":"x","Scope":null}`, `{"query":"x","Limit":0}`, `{"query":"x","scope":{"All_Projects":true}}`, `null`, `{"query":"x","query":"y"}`, `{"query":"x","unknown":1}`, `{"query":null}`, `{"scope":null}`, `{"scope":{"project_id":null}}`, `{"scope":{"project_id":""}}`, `{"scope":{"project_id":"p","all_projects":false}}`, `{"scope":{"unknown":true}}`, `{"context_lines":null}`, `{"context_lines":1.5}`, `{"case_sensitive":null}`, `{"limit":0}`, `{"cursor":""}`} {
		var q SearchRequest
		if Decode([]byte(s), &q) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
