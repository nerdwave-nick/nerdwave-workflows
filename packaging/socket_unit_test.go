package packaging

import (
	"net"
	"os"
	"strings"
	"testing"
)

// TestSocketUnitMatchesServerActivation keeps the shipped socket unit within what
// lit-server accepts on activation: exactly one loopback TCP listener, passed to
// a single long-running service rather than one process per connection.
func TestSocketUnitMatchesServerActivation(t *testing.T) {
	b, err := os.ReadFile("lit.socket")
	if err != nil {
		t.Fatal(err)
	}
	directives := map[string][]string{}
	section := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid line %q", line)
		}
		directives[section+key] = append(directives[section+key], strings.TrimSpace(value))
	}
	listen := directives["[Socket]ListenStream"]
	if len(listen) != 1 {
		t.Fatalf("want exactly one ListenStream, got %v", listen)
	}
	host, port, err := net.SplitHostPort(listen[0])
	if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() || port != "7411" {
		t.Fatalf("ListenStream must be the default loopback listener, got %q", listen[0])
	}
	for key := range directives {
		if strings.HasPrefix(key, "[Socket]Listen") && key != "[Socket]ListenStream" {
			t.Fatalf("unexpected extra listener %s", key)
		}
	}
	if accept := directives["[Socket]Accept"]; len(accept) != 0 && accept[0] != "no" {
		t.Fatalf("Accept=%v would start one server per connection", accept)
	}
	if wanted := directives["[Install]WantedBy"]; len(wanted) != 1 || wanted[0] != "sockets.target" {
		t.Fatalf("WantedBy=%v, want sockets.target", wanted)
	}
}
