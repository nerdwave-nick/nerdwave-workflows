//go:build !linux

package shelltest

import (
	"os/exec"
	"testing"
)

func startOnTerminal(t testing.TB, _ *exec.Cmd, _ string) (func(), func() string) {
	t.Skip("interactive shell tests need a Linux pseudo-terminal")
	return func() {}, func() string { return "" }
}
