package main

import (
	"fmt"
	"io"

	"github.com/nerdwave-nick/nerdwave-workflows/internal/protocol"
	"github.com/nerdwave-nick/nerdwave-workflows/internal/service"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func runCLI(args []string, out, errOut io.Writer) int {
	code := 0
	root := &cobra.Command{Use: "lit-server", SilenceErrors: true, SilenceUsage: true, Version: protocol.ReleaseVersion, CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true}, Short: "Run lit-server in the foreground on a loopback address", Long: `Run lit-server in the foreground. All flags are optional; no installation or background
service is created. Stop with Ctrl-C. The data directory is initialized under an
exclusive lock, and an occupied listen address fails without selecting another port.

Configuration precedence: flags > environment > JSON config > defaults.
The config file defaults to ${XDG_CONFIG_HOME:-$HOME/.config}/lit-server/config.json.
LIT_CONFIG_FILE or --config explicitly selects a file that must exist.
Default data directory: ${XDG_DATA_HOME:-$HOME/.local/share}/lit-server.
Default listener: 127.0.0.1:7411. Listen requires an explicit loopback IP address
(127.0.0.1 or ::1) and a port from 0 to 65535; port 0 asks the OS for a free port.
Existing JSON config must have schema_version: 1. Changes require a restart.`, Example: "  lit-server\n  lit-server --data-dir /tmp/lit-poc --listen 127.0.0.1:7411\n  lit-server --config /path/to/config.json", Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && args[0] == "help" && cmd.ArgsLenAtDash() < 0 {
			return nil
		}
		return cobra.NoArgs(cmd, args)
	}}
	root.Flags().StringArray("config", nil, "Optional JSON config `FILE` (or LIT_CONFIG_FILE)")
	root.Flags().StringArray("data-dir", nil, "Optional authoritative data `DIRECTORY` (or LIT_DATA_DIR)")
	root.Flags().StringArray("listen", nil, "Optional loopback `IP:PORT` (or LIT_LISTEN; default 127.0.0.1:7411)")
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return cmd.Help()
		}
		normalized := []string{}
		for _, name := range []string{"config", "data-dir", "listen"} {
			f := cmd.Flags().Lookup(name)
			if !f.Changed {
				continue
			}
			values := f.Value.(pflag.SliceValue).GetSlice()
			if len(values) != 1 {
				return fmt.Errorf("duplicate flag --%s", name)
			}
			normalized = append(normalized, "--"+name, values[0])
		}
		config, err := service.LoadConfig(normalized)
		if err != nil {
			return err
		}
		code = runService(config, errOut)
		return nil
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	return code
}
