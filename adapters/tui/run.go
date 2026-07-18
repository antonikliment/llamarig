package tui

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
)

func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	uiModel := newModel(ctx)
	opts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)}
	_, err := tea.NewProgram(uiModel, opts...).Run()
	if err != nil {
		return err
	}
	return nil
}
