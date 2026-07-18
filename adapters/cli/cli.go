package cli

import (
	"context"
	"io"
	"os"
)

type Options struct {
	Command, Socket string
	Args            []string
	JSON            bool
	Out             io.Writer
}

type command struct {
	name, socket string
	args         []string
	json         bool
	out          io.Writer
}

func Run(ctx context.Context, opts Options) error {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	return command{
		name:   opts.Command,
		args:   opts.Args,
		socket: opts.Socket,
		json:   opts.JSON,
		out:    out,
	}.run(ctx)
}
