package modelpresets

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"llamarig/config"
	"llamarig/core/modelcatalog"
)

const (
	SourceReady       = "ready"
	SourceChecking    = "checking"
	SourceUnavailable = "unavailable"
)

type SourceStatus struct {
	State string
	Error string
}

type References struct {
	ModelPaths []string
	ModelDirs  []string
}

type CanonicalSection struct {
	Name      string
	Model     string
	ModelsDir string
}

func InspectSource(section Section) SourceStatus {
	if section.Values == nil {
		return SourceStatus{State: SourceUnavailable, Error: "preset has no configured values"}
	}
	sources := []struct {
		path string
		dir  bool
	}{{section.Values["model"], false}, {section.Values["models-dir"], true}}
	configured := false
	for _, source := range sources {
		if strings.TrimSpace(source.path) == "" {
			continue
		}
		configured = true
		if err := requirePath(source.path, source.dir); err != nil {
			return SourceStatus{State: SourceUnavailable, Error: err.Error()}
		}
	}
	if !configured {
		return SourceStatus{State: SourceUnavailable, Error: "preset has no model or models-dir source"}
	}
	return SourceStatus{State: SourceReady}
}

func FindReferences(sections []Section, modelPath string) References {
	return FindReferencesCanonical(CanonicalizeSections(sections), modelPath)
}

func CanonicalizeSections(sections []Section) []CanonicalSection {
	out := make([]CanonicalSection, 0, len(sections))
	for _, section := range sections {
		out = append(out, CanonicalSection{Name: section.Name, Model: sourcePath(section, "model"), ModelsDir: sourcePath(section, "models-dir")})
	}
	return out
}

func FindReferencesCanonical(sections []CanonicalSection, modelPath string) References {
	modelPath = modelcatalog.CanonicalPath(config.ExpandHome(modelPath))
	refs := References{}
	for _, section := range sections {
		if section.Model != "" && section.Model == modelPath {
			refs.ModelPaths = append(refs.ModelPaths, section.Name)
		}
		if section.ModelsDir != "" && modelcatalog.PathContains(section.ModelsDir, modelPath) {
			refs.ModelDirs = append(refs.ModelDirs, section.Name)
		}
	}
	sort.Strings(refs.ModelPaths)
	sort.Strings(refs.ModelDirs)
	return refs
}

func requirePath(raw string, wantDir bool) error {
	path := modelcatalog.CanonicalPath(config.ExpandHome(strings.TrimSpace(raw)))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source %q does not exist", path)
		}
		return fmt.Errorf("inspect source %q: %w", path, err)
	}
	if wantDir && !info.IsDir() {
		return fmt.Errorf("source %q is not a directory", path)
	}
	if !wantDir && !info.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", path)
	}
	return nil
}

func sourcePath(section Section, key string) string {
	if path := strings.TrimSpace(section.Values[key]); path != "" {
		return modelcatalog.CanonicalPath(config.ExpandHome(path))
	}
	return ""
}
