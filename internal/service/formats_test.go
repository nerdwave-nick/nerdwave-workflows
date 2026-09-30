package service

import (
	"fmt"
	"testing"
)

func TestOutputFormatClientLifecycle(t *testing.T) {
	_, h := fixture(t)
	for _, format := range []string{"cli", "markdown", "json", "human"} {
		t.Run(format, func(t *testing.T) {
			status, v := request(t, h, "POST", "/v1/connect", fmt.Sprintf(`{"actor":{"name":"Formats","kind":"human"},"output_format":%q}`, format), "", "")
			if status != 201 {
				t.Fatal(status, v)
			}
			c := v["data"].(map[string]any)["client"].(map[string]any)
			id := c["client_id"].(string)
			if c["output_format"] != format {
				t.Fatal(c)
			}
			status, v = request(t, h, "PATCH", "/v1/clients/"+id, `{"output_format":"markdown"}`, id, `"client:1"`)
			if status != 200 || v["data"].(map[string]any)["output_format"] != "markdown" {
				t.Fatal(status, v)
			}
			rev := fmt.Sprintf(`"client:%.0f"`, v["data"].(map[string]any)["state_revision"])
			status, v = request(t, h, "PATCH", "/v1/clients/"+id, `{"output_format":"bogus"}`, id, rev)
			if status != 422 {
				t.Fatal(status, v)
			}
			status, v = request(t, h, "GET", "/v1/clients/"+id, "", id, "")
			if status != 200 || v["data"].(map[string]any)["output_format"] != "markdown" || v["data"].(map[string]any)["client_id"] != id {
				t.Fatal(status, v)
			}
		})
	}
}
