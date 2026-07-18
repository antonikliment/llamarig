package cmd

import (
	"context"
	"llamarig/bootstrap"
	"llamarig/config"
	"llamarig/platform/process"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func serveCommand() *cobra.Command {
	var detach bool
	cmd := &cobra.Command{
		Use: "serve",
		RunE: func(cmd *cobra.Command, args []string) error {
			if detach {
				return process.StartDetached(config.ProjectName, "serve")
			}
			serve(cmd.Context())
			return nil
		},
	}
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "start in detached mode")
	return cmd
}

func serve(ctx context.Context) {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	svc, err := bootstrap.NewService(ctx, bootstrap.Options{Logger: logger, Env: os.Getenv})
	if err != nil {
		logger.Fatal("initialize service", zap.Error(err))
	}

	process.Run(ctx, logger, "control rpc", svc.Config, func() error {
		logger.Info("starting control rpc", zap.String("socket", svc.ControlRPCSocketPath))
		return svc.ControlRPCServer.Serve(svc.ControlRPCListener)
	}, func(ctx context.Context) error { return shutdown(ctx, logger, svc) })
}

func shutdown(ctx context.Context, logger *zap.Logger, svc *bootstrap.Service) error {
	svc.Close()
	err := svc.ControlRPCServer.Shutdown(ctx)
	if svc.ControlRPCListener != nil {
		_ = svc.ControlRPCListener.Close()
	}
	if svc.ControlRPCSocketPath != "" {
		_ = os.Remove(svc.ControlRPCSocketPath)
	}
	if result, stopErr := svc.Manager.StopOperation(ctx, ""); stopErr != nil {
		logger.Error("stop llama runtime", zap.Error(stopErr))
	} else {
		logger.Info("stopped llama runtime", zap.String("status", result.Status))
	}
	return err
}
