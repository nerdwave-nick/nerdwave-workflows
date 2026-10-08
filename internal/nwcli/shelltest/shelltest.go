// Package shelltest completes command lines in real interactive shells, so
// that completion scripts can be tested the way people use them: bash and zsh
// run on a pseudo-terminal and receive a Tab; fish answers `complete -C`.
package shelltest

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Program is the command a script completes: Name is what the line starts
// with, and running it runs Path with Args before the line's arguments.
type Program struct {
	Name string
	Path string
	Args []string
	Env  []string // added to the shell's environment
	Dir  string   // the shell's working directory
}

// Result is what the shell's completion produced.
type Result struct {
	// Values are the candidates: in bash the texts that would replace the
	// word (COMPREPLY), in zsh and fish the unquoted values.
	Values []string
	// Descriptions are fish's descriptions or zsh's displayed lines.
	Descriptions []string
	// Options are bash's compopt options or zsh's compadd group option
	// (-J sorted, -V in order); "files" when zsh added file names.
	Options []string
}

// Complete loads script into shell and completes line with the cursor at its
// end. It skips the test when the shell is not installed, unless
// NWCLI_REQUIRE_SHELLS=1 (as in CI) makes that a failure.
func Complete(t testing.TB, shell, script string, p Program, line string) Result {
	t.Helper()
	path, err := exec.LookPath(shell)
	if err != nil && os.Getenv("NWCLI_REQUIRE_SHELLS") == "1" {
		t.Fatalf("%s is required but not installed", shell)
	} else if err != nil {
		t.Skipf("%s is not installed", shell)
	}
	dir := t.TempDir()
	scriptFile, out := filepath.Join(dir, "script"), filepath.Join(dir, "out")
	if err := os.WriteFile(scriptFile, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "SHELLTEST_BIN="+p.Path, "SHELLTEST_ARGS="+strings.Join(p.Args, " "),
		"SHELLTEST_SCRIPT="+scriptFile, "SHELLTEST_OUT="+out, "SHELLTEST_LINE="+line, "COLUMNS=200", "LINES=50", "TERM=xterm")
	env = append(env, p.Env...)
	if shell == "fish" {
		cmd := exec.Command(path, "--no-config", "-c", strings.ReplaceAll(fishDriver, "PROG", p.Name))
		cmd.Env, cmd.Dir = env, p.Dir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("fish: %v: %s", err, output)
		}
		return parse(string(output))
	}
	driver, ok := map[string]string{"bash": bashDriver, "zsh": zshDriver}[shell]
	if !ok {
		t.Fatalf("unsupported shell %q", shell)
	}
	args := map[string][]string{"bash": {"--norc", "--noprofile", "-i"}, "zsh": {"-f", "-i"}}[shell]
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Dir = env, p.Dir
	stop, screen := startOnTerminal(t, cmd, strings.ReplaceAll(driver, "PROG", p.Name)+line+"\t")
	defer stop()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(out); err == nil && strings.HasSuffix(string(b), "DONE\n") {
			return parse(strings.TrimSuffix(string(b), "DONE\n"))
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s produced no completion for %q; terminal:\n%s", shell, line, screen())
	return Result{}
}

// parse reads the drivers' output: one record per line, tagged V (value),
// D (description) or O (option).
func parse(output string) Result {
	var r Result
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "V"):
			r.Values = append(r.Values, line[1:])
		case strings.HasPrefix(line, "D"):
			r.Descriptions = append(r.Descriptions, line[1:])
		case strings.HasPrefix(line, "O"):
			r.Options = append(r.Options, line[1:])
		}
	}
	return r
}

// The drivers define PROG, load the script, wrap the registered completion
// function to record its outcome in $SHELLTEST_OUT, and end ready for input.
const fishDriver = `function PROG; "$SHELLTEST_BIN" (string split -n ' ' -- $SHELLTEST_ARGS) $argv; end
source "$SHELLTEST_SCRIPT"
for line in (complete -C "$SHELLTEST_LINE")
    set -l parts (string split -m 1 \t -- $line)
    printf 'V%s\n' $parts[1]
    set -q parts[2]; and printf 'D%s\n' $parts[2]
end
true
`

const bashDriver = `PS1=; PROG() { "$SHELLTEST_BIN" $SHELLTEST_ARGS "$@"; }
source "$SHELLTEST_SCRIPT"
__shelltest_function=$(complete -p PROG | sed -n 's/.* -F \([^ ]*\) .*/\1/p')
__shelltest() {
    "$__shelltest_function" "$@"; local ret=$?
    local -a options=($(compopt | grep -o -- '-o [a-z]*' | cut -c4-))
    { ((${#COMPREPLY[@]})) && printf 'V%s\n' "${COMPREPLY[@]}"; ((${#options[@]})) && printf 'O%s\n' "${options[@]}"; echo DONE; } > "$SHELLTEST_OUT.tmp"
    mv "$SHELLTEST_OUT.tmp" "$SHELLTEST_OUT"
    return $ret
}
complete -F __shelltest PROG
`

const zshDriver = `PS1=; autoload -Uz compinit; compinit -u -d /dev/null
PROG() { "$SHELLTEST_BIN" ${=SHELLTEST_ARGS} "$@"; }
source "$SHELLTEST_SCRIPT"
__shelltest_function=$_comps[PROG]
compadd() {
    local -a args=("$@") __shelltest_displays; local i
    for (( i = 1; i <= $#args; i++ )); do
        case $args[i] in
        -d) __shelltest_displays=("${(@P)args[i+1]}") ;;
        -a) print -rl -- "${(@)${(@P)args[i+1]}/#/V}" >> "$SHELLTEST_OUT.tmp" ;;
        -J|-V) print -r -- "O$args[i]" >> "$SHELLTEST_OUT.tmp" ;;
        -[^-]*f*|-f) print -r -- "Ofiles" >> "$SHELLTEST_OUT.tmp" ;;
        --) print -rl -- "${(@)${(@)args[i+1,-1]}/#/V}" >> "$SHELLTEST_OUT.tmp"; break ;;
        esac
    done
    (( $#__shelltest_displays )) && print -rl -- "${(@)__shelltest_displays/#/D}" >> "$SHELLTEST_OUT.tmp"
    builtin compadd "$@"
}
__shelltest() { "$__shelltest_function" "$@"; local ret=$?; print DONE >> "$SHELLTEST_OUT.tmp"; mv "$SHELLTEST_OUT.tmp" "$SHELLTEST_OUT"; return $ret; }
compdef __shelltest PROG
: > "$SHELLTEST_OUT.tmp"
`
