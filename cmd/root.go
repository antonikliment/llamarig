package cmd

import (
	"context"
	"llamarig/config"
	"llamarig/internal/buildinfo"

	"github.com/spf13/cobra"
)

func Execute() error {
	rootCmd := NewRootCommand()
	return rootCmd.ExecuteContext(context.Background())
}

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{Use: config.ProjectName, Version: buildinfo.Version, CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true}, RunE: func(cmd *cobra.Command, args []string) error { return runTUI(cmd) }}

	rootCmd.AddCommand(serveCommand())
	rootCmd.AddCommand(gatewayCommand())
	rootCmd.AddCommand(setupCommand)
	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(downCommand)
	rootCmd.AddCommand(logsCommand(config.ProjectName, true))
	rootCmd.AddCommand(tuiCommand)
	rootCmd.AddCommand(cliCommands()...)

	return rootCmd
}
