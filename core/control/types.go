package control

import "time"

type RuntimeInfo struct {
	Router           Info     `json:"router"`
	PresetsCount     int      `json:"presets_count"`
	DefaultPreset    string   `json:"default_preset,omitempty"`
	AutostartPresets []string `json:"autostart_presets,omitempty"`
}

type Info struct {
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

type RuntimeStatus struct {
	State     string          `json:"state"`
	Detail    string          `json:"detail,omitempty"`
	CheckedAt time.Time       `json:"checked_at"`
	Presets   []RuntimePreset `json:"presets,omitempty"`
}

type RuntimePreset struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Ready bool   `json:"ready"`
}
