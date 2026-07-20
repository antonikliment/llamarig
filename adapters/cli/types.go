package cli

import "io"

type command struct {
	name, socket string
	args         []string
	json         bool
	out          io.Writer
}
