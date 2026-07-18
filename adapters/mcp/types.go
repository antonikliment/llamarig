package mcp

type emptyInput struct{}

type runtimeInput struct {
	Preset string `json:"preset,omitempty" jsonschema:"model preset name"`
}

type presetNameInput struct {
	Name string `json:"name" jsonschema:"model preset name"`
}

type presetPutInput struct {
	Name       string            `json:"name" jsonschema:"model preset name"`
	Entries    map[string]string `json:"entries" jsonschema:"models.ini key/value entries"`
	CreateOnly bool              `json:"create_only,omitempty" jsonschema:"fail if the preset already exists"`
}
