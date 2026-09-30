package clientendpoint

import "testing"

func TestRejectInvalidSelectedOrigin(t *testing.T) {
	for _, invalid := range []string{"bad", "http://", "https://example.com/path", "https://example.com?x=1", "https://user@example.com", "https://example.com#fragment"} {
		for _, selected := range []string{"flag", "environment", "mapping"} {
			t.Run(selected+"/"+invalid, func(t *testing.T) {
				explicit, environment, mapping := "", "", ""
				switch selected {
				case "flag":
					explicit = invalid
					environment = "https://valid.example"
				case "environment":
					environment = invalid
					mapping = "https://valid.example"
				case "mapping":
					mapping = invalid
				}
				if got, err := Resolve(explicit, selected == "flag", environment, mapping); err == nil {
					t.Fatalf("accepted %q as %q", invalid, got)
				}
			})
		}
	}
}
