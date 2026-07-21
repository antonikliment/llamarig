package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"llamarig/adapters/tui"
	"llamarig/config"
	"llamarig/core/setup"
	"llamarig/platform/audit"
	"llamarig/platform/process"
	"time"

	"github.com/spf13/cobra"
)

// setupEnsure runs the first-run setup wizard when no config exists. It is a
// package-level indirection so tests can observe the bare/TUI path invoking it.
var setupEnsure = setup.Ensure

var setupCommand = &cobra.Command{
	Use: "setup",
	RunE: func(cmd *cobra.Command, args []string) error {
		return setup.Rerun(cmd.Context())
	},
}

var downCommand = &cobra.Command{
	Use: "down",
	RunE: func(cmd *cobra.Command, args []string) error {
		return process.StopDetached(config.ProjectName)
	},
}

func logsCommand(name string, sourceFlag bool) *cobra.Command {
	var source string
	var lines int
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail service logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logName, err := commandLogName(name, source)
			if err != nil {
				return err
			}
			return printLogTail(cmd.Context(), logName, lines, follow, cmd.OutOrStdout())
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 200, "number of lines to show")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow appended lines")
	if sourceFlag {
		cmd.Flags().StringVar(&source, "source", "control", "log source: control or gateway")
		cmd.AddCommand(logArchiveCommand())
	}
	return cmd
}

func logArchiveCommand() *cobra.Command {
	archive := &cobra.Command{Use: "archive", Short: "Manage log archives"}
	archive.AddCommand(
		&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: listLogArchives},
		archiveShowCommand(),
		&cobra.Command{Use: "delete <id>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error { return audit.DeleteArchive(args[0]) }},
		archiveClearCommand(),
	)
	return archive
}

func archiveShowCommand() *cobra.Command {
	var lines int
	cmd := &cobra.Command{Use: "show <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		text, err := audit.TailArchive(args[0], lines)
		if err != nil {
			return err
		}
		_, err = io.WriteString(cmd.OutOrStdout(), text)
		return err
	}}
	cmd.Flags().IntVarP(&lines, "lines", "n", 200, "number of lines to show")
	return cmd
}

func archiveClearCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "clear", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !yes {
			return fmt.Errorf("refusing to clear all log archives without --yes")
		}
		deleted, err := audit.ClearArchives()
		if err == nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted %d log archive(s)\n", deleted)
		}
		return err
	}}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion of all archives")
	return cmd
}

func listLogArchives(cmd *cobra.Command, _ []string) error {
	archives, err := audit.ListArchives()
	if err != nil {
		return err
	}
	for _, archive := range archives {
		service := archive.Service
		if service == config.ProjectName {
			service = "control"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\n", archive.ID, service, archive.SizeBytes, archive.ArchivedAt.Format(time.RFC3339Nano))
	}
	return nil
}

func commandLogName(defaultName, source string) (string, error) {
	if source == "" {
		return defaultName, nil
	}
	switch source {
	case "control":
		return config.ProjectName, nil
	case "gateway":
		return "gateway", nil
	default:
		return "", fmt.Errorf("log source must be control or gateway")
	}
}

func printLogTail(ctx context.Context, name string, lines int, follow bool, out io.Writer) error {
	if follow {
		return audit.FollowLog(ctx, name, lines, out, 500*time.Millisecond)
	}
	text, err := audit.TailLogLines(name, lines)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

var tuiCommand = &cobra.Command{
	Use: "tui",
	//Short: "Run the interactive agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(cmd)
	},
}

// runTUI launches the TUI entrypoint. Shared by the bare root command and the
// explicit TUI subcommand.
func runTUI(cmd *cobra.Command) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	// Run the first-run setup wizard on the real TTY before bubbletea takes
	// over stdin/stdout. Ensure no-ops when a config already exists, so this is
	// safe and idempotent. A cancelled wizard exits cleanly.
	if err := setupEnsure(ctx); err != nil {
		if errors.Is(err, setup.ErrCancelled) {
			return nil
		}
		return err
	}
	return tui.Run(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
}
