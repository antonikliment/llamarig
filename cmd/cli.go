package cmd

import (
	"llamarig/adapters/cli"
	"llamarig/config"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func cliCommands() []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(cli.Commands()))
	for _, spec := range cli.Commands() {
		socket, jsonOut := "", false
		cmd := &cobra.Command{
			Use:               strings.TrimSpace(spec.Name + " " + spec.Usage),
			Short:             spec.Short,
			Args:              argsValidator(spec),
			ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cmd *cobra.Command, args []string) error {
				socketPath := socket
				if !cmd.Flags().Changed("socket") {
					socketPath = os.Getenv(config.ProjectSocketEnv)
				}
				return cli.Run(cmd.Context(), cli.Options{
					Command: spec.Name,
					Args:    args,
					Socket:  socketPath,
					JSON:    jsonOut,
					Out:     cmd.OutOrStdout(),
				})
			},
		}
		cmd.Flags().StringVar(&socket, "socket", "", config.ProjectDisplayName+" control Unix socket")
		cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
		cmds = append(cmds, cmd)
	}
	return cmds
}

func argsValidator(spec cli.CommandSpec) cobra.PositionalArgs {
	switch {
	case spec.MinArgs == 0 && spec.MaxArgs == 0:
		return cobra.NoArgs
	case spec.MinArgs == spec.MaxArgs:
		return cobra.ExactArgs(spec.MinArgs)
	case spec.MaxArgs < 0:
		return cobra.MinimumNArgs(spec.MinArgs)
	default:
		return cobra.RangeArgs(spec.MinArgs, spec.MaxArgs)
	}
}
