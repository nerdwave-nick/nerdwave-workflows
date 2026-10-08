package nwcli

import (
	"embed"
	"fmt"
	"io"
	"regexp"
	"strings"
)

//go:embed scripts
var scripts embed.FS

// Shells lists the shells WriteScript supports.
var Shells = []string{"bash", "fish", "zsh"}

var scriptFiles = map[string]string{"bash": "scripts/bash.sh", "fish": "scripts/fish.fish", "zsh": "scripts/zsh.zsh"}

// WriteScript writes the completion script of shell for program. The script
// answers each completion by running `program command WORDS... CURRENT`,
// where command serves Complete and WriteCompletion (conventionally
// "__complete"), and honours every Directive.
func WriteScript(w io.Writer, shell, program, command string) error {
	file, ok := scriptFiles[shell]
	if !ok {
		return fmt.Errorf("unsupported shell %q; use bash, fish or zsh", shell)
	}
	text, err := scripts.ReadFile(file)
	if err != nil {
		return err
	}
	id := regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(program, "_")
	_, err = io.WriteString(w, strings.NewReplacer("{{PROG}}", program, "{{ID}}", id, "{{COMPLETE}}", command).Replace(string(text)))
	return err
}
