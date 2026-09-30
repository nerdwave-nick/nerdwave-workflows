package protocol

import "testing"

func TestStrictJSON(t *testing.T) {
	var v struct {
		Actor Actor `json:"actor"`
	}
	for _, s := range []string{`{"actor":{"name":"A","name":"B","kind":"human"}}`, `{"actor":{"unknown":1}}`, `{"actor":{"name":"A"}} {}`, `{"actor":{"name":"A"},"actor":{}}`} {
		if e := Decode([]byte(s), &v); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	if e := Decode([]byte(`{"actor":{"kind":"human","name":"A"}}`), &v); e != nil {
		t.Fatal(e)
	}
}
