package cli

import (
	"cmp"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func (c command) printJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.out, string(data))
	return err
}

func (c command) printInfo(v *controlv1.RuntimeInfo) error {
	_, err := fmt.Fprintf(c.out, "status: %s\ndefault: %s\npresets: %d\nautostart: %s\n",
		v.GetRouter().GetStatus(), dash(v.GetDefaultPreset()), v.GetPresetsCount(), list(v.GetAutostartPresets()))
	return err
}

func (c command) printStatus(v *controlv1.RuntimeStatus) error {
	_, err := fmt.Fprintf(c.out, "status: %s\ndetail: %s\n", v.GetState(), dash(v.GetDetail()))
	return err
}

func (c command) printPresets(presets []*controlv1.ModelPreset) error {
	w := tabwriter.NewWriter(c.out, 0, 0, 2, ' ', 0)
	for _, p := range presets {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", p.GetName(), cmp.Or(presetEntry(p, "model"), presetEntry(p, "models-dir"), "-")); err != nil {
			return err
		}
	}
	return w.Flush()
}

func presetEntry(preset *controlv1.ModelPreset, key string) string {
	for _, entry := range preset.GetEntries() {
		if entry.GetKey() == key {
			return entry.GetValue()
		}
	}
	return ""
}

func (c command) printPreset(preset *controlv1.ModelPreset) error {
	enc := json.NewEncoder(c.out)
	enc.SetIndent("", "  ")
	return enc.Encode(preset)
}

func (c command) printAction(result *controlv1.CommandResult) error {
	_, err := fmt.Fprintf(c.out, "%s: %s%s\n", result.GetAction(), result.GetStatus(), suffix(result.GetMessage()))
	return err
}
