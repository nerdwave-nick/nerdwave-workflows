package nwcli

import (
	"strings"
	"testing"
)

func TestWriteScriptFillsInTheProgram(t *testing.T) {
	for _, shell := range Shells {
		var b strings.Builder
		if err := WriteScript(&b, shell, "my-tool", "__complete"); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		script := b.String()
		if strings.Contains(script, "{{") || !strings.Contains(script, "my-tool") || !strings.Contains(script, "__complete") || !strings.Contains(script, "_my_tool") {
			t.Errorf("%s script not filled in:\n%s", shell, script)
		}
	}
	if err := WriteScript(&strings.Builder{}, "powershell", "my-tool", "__complete"); err == nil || err.Error() != `unsupported shell "powershell"; use bash, fish or zsh` {
		t.Errorf("unsupported shell: %v", err)
	}
	if strings.Join(Shells, ",") != "bash,fish,zsh" {
		t.Errorf("shells: %v", Shells)
	}
}
