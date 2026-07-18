package cmd

import (
	"context"
	"llamarig/bootstrap"
	"llamarig/platform/process"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func gatewayCommand() *cobra.Command {
	var foreground bool
	var gatewayCmd = &cobra.Command{
		Use: "gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				gateway(cmd.Context())
				return nil
			}
			return process.StartDetached("gateway", "gateway", "--foreground")
		},
	}
	gatewayCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "run in foreground")
	gatewayCmd.AddCommand(&cobra.Command{Use: "down", RunE: func(cmd *cobra.Command, args []string) error { return process.StopDetached("gateway") }})
	gatewayCmd.AddCommand(logsCommand("gateway", false))
	return gatewayCmd
}

func gateway(ctx context.Context) {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()
	svc, err := bootstrap.NewGateway(ctx, bootstrap.Options{Logger: logger, Env: os.Getenv})
	if err != nil {
		logger.Fatal("initialize gateway", zap.Error(err))
	}
	process.Run(ctx, logger, "gateway", svc.Config, func() error {
		logger.Info("starting gateway", zap.String("listen_addr", svc.Config.ListenAddr))
		return svc.HTTPServer.ListenAndServe()
	}, svc.HTTPServer.Shutdown)
}
