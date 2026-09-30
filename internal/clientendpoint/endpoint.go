// Package clientendpoint defines endpoint selection shared by the CLI and host adapter.
package clientendpoint

import (
	"fmt"
	"net/url"
	"strings"
)

const Default = "http://127.0.0.1:7411"

// Resolve preserves an explicit endpoint, including invalid or empty values, so
// a typo cannot silently route an operation to a different tracker.
func Resolve(explicit string, supplied bool, environment, remembered string) (string, error) {
	endpoint := explicit
	if !supplied {
		endpoint = environment
		if endpoint == "" {
			endpoint = remembered
		}
		if endpoint == "" {
			endpoint = Default
		}
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("endpoint must be an HTTP(S) origin")
	}
	return strings.TrimRight(endpoint, "/"), nil
}
