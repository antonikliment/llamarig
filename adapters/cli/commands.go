package cli

import (
	"context"
	"fmt"
	"slices"

	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

type commandHandler func(c command, ctx context.Context, client controlv1connect.ControlServiceClient) error

type CommandSpec struct {
	Name, Usage, Short string
	MinArgs, MaxArgs   int
	handler            commandHandler
}

// commandRegistry is the single source of truth for CLI command names and
// their metadata and handlers, in registration order. cmd/cli.go registers
// cobra commands from Commands; Run dispatches through the same table.
var commandRegistry = []CommandSpec{
	{Name: "info", Short: "Show control daemon information", MaxArgs: 0, handler: (command).runInfo},
	{Name: "status", Short: "Show runtime status", MaxArgs: 0, handler: (command).runStatus},
	{Name: "presets", Short: "List presets", MaxArgs: 0, handler: (command).runPresets},
	{Name: "preset", Usage: "<name>", Short: "Show a preset", MinArgs: 1, MaxArgs: 1, handler: (command).runPreset},
	{Name: "start", Usage: "[preset]", Short: "Start the requested or default preset", MaxArgs: 1, handler: (command).runAction},
	{Name: "stop", Usage: "[preset]", Short: "Stop the requested or all running presets", MaxArgs: 1, handler: (command).runAction},
	{Name: "restart", Usage: "[preset]", Short: "Restart the requested preset", MaxArgs: 1, handler: (command).runAction},
}

func Commands() []CommandSpec { return slices.Clone(commandRegistry) }

func lookupCommand(name string) (CommandSpec, bool) {
	for _, spec := range commandRegistry {
		if spec.Name == name {
			return spec, true
		}
	}
	return CommandSpec{}, false
}

func (c command) run(ctx context.Context) error {
	entry, ok := lookupCommand(c.name)
	if !ok {
		return fmt.Errorf("unknown command %q", c.name)
	}
	if len(c.args) < entry.MinArgs || entry.MaxArgs >= 0 && len(c.args) > entry.MaxArgs {
		return fmt.Errorf("invalid arguments for %q: received %d", c.name, len(c.args))
	}
	client, err := c.controlClient()
	if err != nil {
		return err
	}
	return entry.handler(c, ctx, client)
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
