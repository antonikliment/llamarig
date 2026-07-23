package cmd

import (
	"fmt"

	"llamarig/core/llamainstall"

	"github.com/spf13/cobra"
)

func llamaCommand() *cobra.Command {
	command := &cobra.Command{Use: "llama", Short: "Manage llama.cpp"}
	command.AddCommand(llamaInstallCommand(false), llamaInstallCommand(true))
	return command
}

func llamaInstallCommand(upgrade bool) *cobra.Command {
	var backend string
	var source bool
	var jobs int
	name := "install"
	if upgrade {
		name = "upgrade"
	}
	command := &cobra.Command{
		Use:   name,
		Short: name + " managed llama.cpp",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			options := llamainstall.Options{
				Backend: llamainstall.Backend(backend), Source: source, Jobs: jobs,
				BackendSet: cmd.Flags().Changed("backend"), SourceSet: cmd.Flags().Changed("source"),
				Progress: cmd.ErrOrStderr(),
			}
			operation := llamainstall.Install
			if upgrade {
				operation = llamainstall.Upgrade
			}
			executable, err := operation(cmd.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), executable)
			return err
		},
	}
	command.Flags().StringVar(&backend, "backend", "auto", "backend: auto, cpu, cuda, rocm, vulkan, or metal")
	command.Flags().BoolVar(&source, "source", false, "build from source")
	command.Flags().IntVarP(&jobs, "jobs", "j", 4, "parallel source build jobs")
	return command
}
