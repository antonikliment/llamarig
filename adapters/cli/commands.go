package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"llamarig/config"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

type commandHandler func(c command, ctx context.Context, client controlv1connect.ControlServiceClient) error

type commandSpec struct {
	name, usage, short string
	args               cobra.PositionalArgs
	handler            commandHandler
}

// commandRegistry defines CLI metadata and handlers in display order.
var commandRegistry = []commandSpec{
	{name: "info", short: "Show control daemon information", args: cobra.NoArgs, handler: (command).runInfo},
	{name: "status", short: "Show runtime status", args: cobra.NoArgs, handler: (command).runStatus},
	{name: "presets", short: "List presets", args: cobra.NoArgs, handler: (command).runPresets},
	{name: "preset", usage: "<name>", short: "Show a preset", args: cobra.ExactArgs(1), handler: (command).runPreset},
	{name: "start", usage: "[preset]", short: "Start the requested or default preset", args: cobra.MaximumNArgs(1), handler: (command).runAction},
	{name: "stop", usage: "[preset]", short: "Stop the requested or all running presets", args: cobra.MaximumNArgs(1), handler: (command).runAction},
	{name: "restart", usage: "[preset]", short: "Restart the requested preset", args: cobra.MaximumNArgs(1), handler: (command).runAction},
}

func Commands() []*cobra.Command {
	commands := make([]*cobra.Command, 0, len(commandRegistry))
	for _, spec := range commandRegistry {
		socket, jsonOut := "", false
		cobraCmd := &cobra.Command{
			Use: strings.TrimSpace(spec.name + " " + spec.usage), Short: spec.short, Args: spec.args, ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cobraCmd *cobra.Command, args []string) error {
				socketPath := socket
				if !cobraCmd.Flags().Changed("socket") {
					socketPath = os.Getenv(config.ProjectSocketEnv)
				}
				c := command{name: spec.name, args: args, socket: socketPath, json: jsonOut, out: cobraCmd.OutOrStdout()}
				client, err := c.controlClient()
				if err != nil {
					return err
				}
				return spec.handler(c, cobraCmd.Context(), client)
			},
		}
		cobraCmd.Flags().StringVar(&socket, "socket", "", config.ProjectDisplayName+" control Unix socket")
		cobraCmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
		commands = append(commands, cobraCmd)
	}
	return commands
}

// respond prints jsonVal if JSON output was requested, else calls humanFn.
func (c command) respond(jsonVal any, humanFn func() error) error {
	if c.json {
		return c.printJSON(jsonVal)
	}
	return humanFn()
}

// okRespond handles the standard RPC pattern: propagate err or delegate to respond.
func (c command) okRespond(err error, jsonVal any, printFn func() error) error {
	if err != nil {
		return err
	}
	return c.respond(jsonVal, printFn)
}

func (c command) runInfo(ctx context.Context, client controlv1connect.ControlServiceClient) error {
	out, err := client.GetInfo(ctx, &controlv1.GetInfoRequest{})
	return c.okRespond(err, out.GetInfo(), func() error { return c.printInfo(out.GetInfo()) })
}

func (c command) runStatus(ctx context.Context, client controlv1connect.ControlServiceClient) error {
	out, err := client.GetRuntimeStatus(ctx, &controlv1.GetRuntimeStatusRequest{})
	return c.okRespond(err, out.GetStatus(), func() error { return c.printStatus(out.GetStatus()) })
}

func (c command) runPresets(ctx context.Context, client controlv1connect.ControlServiceClient) error {
	out, err := client.ListPresets(ctx, &controlv1.ListPresetsRequest{})
	return c.okRespond(err, map[string]any{"presets": out.GetPresets()}, func() error { return c.printPresets(out.GetPresets()) })
}

func (c command) runPreset(ctx context.Context, client controlv1connect.ControlServiceClient) error {
	out, err := client.GetPreset(ctx, &controlv1.GetPresetRequest{Name: c.args[0]})
	return c.okRespond(err, map[string]any{"preset": out.GetPreset()}, func() error { return c.printPreset(out.GetPreset()) })
}

func (c command) runAction(ctx context.Context, client controlv1connect.ControlServiceClient) error {
	target := ""
	if len(c.args) == 1 {
		target = c.args[0]
	}
	out, err := c.callAction(ctx, client, target)
	if err != nil {
		return err
	}
	return c.respond(map[string]any{"result": out.GetResult()}, func() error { return c.printAction(out.GetResult()) })
}

func (c command) callAction(ctx context.Context, client controlv1connect.ControlServiceClient, target string) (*controlv1.CommandResponse, error) {
	req := &controlv1.RuntimeTargetRequest{Target: target}
	switch c.name {
	case "start":
		return client.StartRuntime(ctx, req)
	case "stop":
		return client.StopRuntime(ctx, req)
	case "restart":
		return client.RestartRuntime(ctx, req)
	default:
		return nil, fmt.Errorf("unknown action %q", c.name)
	}
}
