---
status: accepted
---

# Hand-rolled command line (nwcli) instead of cobra

lit's argument grammar is order-dependent: an item flag such as `--issue` begins
an atomic item and owns its fields (`--content`, `--label`, …), so
`issues create --issue A --content x --issue B --content y` is valid while a
second `--content` within one item is not. Cobra and pflag collect flags without
order, so lit ran two parsers: cobra for routing, help and completion, then its
own grammar on the raw arguments. They kept disagreeing (every flag declared as a
repeatable array, hidden shadow copies of global flags, a help-alias rewrite,
cobra's one-letter long-flag lookup bug, completion re-offering used flags), so
we replaced cobra with a grammar of our own, `internal/nwcli`, which lit's
parsers, routing, help, error reporting and completion all read.

`nwcli` is built as a future standalone library: it imports only the standard
library (enforced by a test), takes the program name as a parameter, and lit
plugs in its record completion, help requirements and error envelope. It stays
`internal` until a second user exists; extracting it into its own module is a
separate decision. `lit-server` uses it too.

## Considered options

- **Keep cobra, declare repeatability per command.** Cheaper, but cobra still
  cannot express items, so two parsers and their workarounds would remain.
- **Replace cobra but keep its generated completion scripts.** The scripts only
  read a line protocol and could have been embedded unchanged; we wrote our own
  bash, zsh and fish scripts instead to own their behaviour (word splitting at
  `:`, quoting, file fallback) and test them in real shells.

## Consequences

- Help output keeps cobra's layout and is pinned by `internal/cli/testdata/help.golden`.
- Shell completion covers bash 4.4+, zsh 5.8+ and fish 3.4+; **PowerShell
  completion was dropped**, so the Windows client has none.
- Completion scripts are tested in real interactive shells (`nwcli/shelltest`);
  CI installs zsh and fish and requires those tests to run.
