package tabs

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	NextPanel        key.Binding
	PreviousPanel    key.Binding
	PreviousAction   key.Binding
	NextAction       key.Binding
	RunAction        key.Binding
	ServicesTab      key.Binding
	ModelsTab        key.Binding
	SystemTab        key.Binding
	LogsTab          key.Binding
	Refresh          key.Binding
	Quit             key.Binding
	Up               key.Binding
	Down             key.Binding
	ToggleAutostart  key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		NextPanel:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab/Shift+Tab", "Focus")),
		PreviousPanel:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("Tab/Shift+Tab", "Focus")),
		PreviousAction: key.NewBinding(key.WithKeys("left"), key.WithHelp("←/→", "Select")),
		NextAction:     key.NewBinding(key.WithKeys("right"), key.WithHelp("←/→", "Select")),
		RunAction:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Run")),
		ServicesTab:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1/2/3/4", "Switch tab")),
		ModelsTab:      key.NewBinding(key.WithKeys("2"), key.WithHelp("1/2/3/4", "Switch tab")),
		SystemTab:      key.NewBinding(key.WithKeys("3"), key.WithHelp("1/2/3/4", "Switch tab")),
		LogsTab:        key.NewBinding(key.WithKeys("4"), key.WithHelp("1/2/3/4", "Switch tab")),
		Refresh:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Refresh")),
		Quit:            key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "Quit")),
		Up:              key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "Navigate")),
		Down:            key.NewBinding(key.WithKeys("down"), key.WithHelp("↑/↓", "Navigate")),
		ToggleAutostart: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Toggle autostart")),
	}
}

func helpText(b key.Binding) string { h := b.Help(); return h.Key + " " + h.Desc }

func quickHelpLines(keys KeyMap) []string {
	return []string{
		helpText(keys.NextPanel),
		helpText(keys.NextAction),
		helpText(keys.RunAction),
		helpText(keys.ServicesTab),
		helpText(keys.Refresh),
		helpText(keys.Quit),
		helpText(keys.ToggleAutostart),
	}
}
