package modelpresets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"llamarig/platform/filedoc"
)

var (
	ErrInvalid  = errors.New("invalid preset")
	ErrExists   = errors.New("preset already exists")
	ErrNotFound = errors.New("preset not found")
)

type Section struct {
	Name   string
	Values map[string]string
}

type Store struct {
	mu   sync.RWMutex
	path string
}

func NewStore(path string) *Store { return &Store{path: filepath.Clean(path)} }

func (s *Store) Root() string { return s.path }

func (s *Store) List(ctx context.Context) ([]Section, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sections, _, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	delete(sections, "*")
	out := make([]Section, 0, len(sections))
	for _, section := range sections {
		out = append(out, section)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Global returns the `[*]` cascade section, or an empty section when absent.
func (s *Store) Global(ctx context.Context) (Section, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sections, _, err := s.read(ctx)
	if err != nil {
		return Section{}, err
	}
	if section, ok := sections["*"]; ok {
		return section, nil
	}
	return Section{Name: "*", Values: map[string]string{}}, nil
}

func (s *Store) Get(ctx context.Context, name string) (Section, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sections, _, err := s.read(ctx)
	if err != nil {
		return Section{}, err
	}
	section, ok := sections[name]
	if !ok {
		return Section{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return section, nil
}

func (s *Store) Put(ctx context.Context, section Section, createOnly bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateSectionName(section.Name); err != nil {
		return err
	}
	values := make(map[string]string, len(section.Values))
	for key, value := range section.Values {
		canonical, err := canonicalKey(key)
		if err != nil {
			return err
		}
		if _, exists := values[canonical]; exists {
			return fmt.Errorf("%w: duplicate models.ini key %q", ErrInvalid, canonical)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%w: value for key %q contains a newline", ErrInvalid, key)
		}
		values[canonical] = value
	}
	sections, content, err := s.read(ctx)
	if err != nil {
		return err
	}
	_, exists := sections[section.Name]
	if createOnly && exists {
		return fmt.Errorf("%w: %q", ErrExists, section.Name)
	}
	if exists {
		content = removeSection(content, section.Name)
	}
	content = strings.TrimRight(content, "\r\n") + renderSection(Section{Name: section.Name, Values: values})
	return s.write(content)
}

func (s *Store) Delete(ctx context.Context, name string) error {
	if err := validateSectionName(name); err != nil {
		return err
	}
	if name == "*" {
		return fmt.Errorf("%w: global preset section cannot be deleted", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sections, content, err := s.read(ctx)
	if err != nil {
		return err
	}
	if _, exists := sections[name]; !exists {
		return nil
	}
	return s.write(removeSection(content, name))
}

func validateSectionName(name string) error {
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "[]\r\n;#=") {
		return fmt.Errorf("%w: name %q", ErrInvalid, name)
	}
	return nil
}

//nolint:gocognit,gocyclo // INI grammar stays local; adding parser dependency buys little.
func (s *Store) read(_ context.Context) (map[string]Section, string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Section{}, "version = 1\n", nil
	}
	if err != nil {
		return nil, "", err
	}
	sections := map[string]Section{}
	current := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			if !strings.HasSuffix(trimmed, "]") {
				return nil, "", fmt.Errorf("%w: malformed section header %q", ErrInvalid, trimmed)
			}
			current = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			if err := validateSectionName(current); err != nil {
				return nil, "", err
			}
			if _, exists := sections[current]; exists {
				return nil, "", fmt.Errorf("%w: duplicate models.ini section %q", ErrInvalid, current)
			}
			sections[current] = Section{Name: current, Values: map[string]string{}}
			continue
		}
		if current == "" {
			continue
		} // version and future file-level keys
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, "", fmt.Errorf("%w: expected key = value, got %q", ErrInvalid, trimmed)
		}
		canonical, err := canonicalKey(strings.TrimSpace(key))
		if err != nil {
			return nil, "", err
		}
		if comment := inlineCommentPattern.FindStringIndex(value); comment != nil {
			value = value[:comment[0]]
		}
		sections[current].Values[canonical] = strings.TrimSpace(value)
	}
	return sections, string(data), nil
}

func (s *Store) write(content string) error {
	_, err := filedoc.WriteFile(s.path, strings.TrimRight(content, "\r\n")+"\n", filedoc.WriteOptions{Perm: 0o600})
	return err
}

var keyAliases = map[string]string{
	"batch":            "batch-size",
	"ubatch":           "ubatch-size",
	"n-parallel":       "parallel",
	"endpoint-metrics": "metrics",
	"endpoint-props":   "props",
	"endpoint-slots":   "slots",
	"think":            "reasoning-format",
	"think-budget":     "reasoning-budget",
	"models_dir":       "models-dir",
	"models_preset":    "models-preset",
	"models_max":       "models-max",
}

var canonicalKeyPattern = regexp.MustCompile(`^[a-z_][a-z0-9_.-]*$`)
var inlineCommentPattern = regexp.MustCompile(`[ \t][;#]`)

func canonicalKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", fmt.Errorf("%w: models.ini contains empty key", ErrInvalid)
	}
	if strings.HasPrefix(trimmed, "LLAMA_ARG_") {
		if trimmed != strings.ToUpper(trimmed) {
			return "", fmt.Errorf("%w: models.ini environment key %q must use uppercase", ErrInvalid, key)
		}
		trimmed = strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(trimmed, "LLAMA_ARG_"), "_", "-"))
	} else if trimmed != strings.ToLower(trimmed) {
		return "", fmt.Errorf("%w: models.ini key %q must use lowercase CLI form or exact LLAMA_ARG_* form", ErrInvalid, key)
	}
	if !canonicalKeyPattern.MatchString(trimmed) {
		return "", fmt.Errorf("%w: invalid models.ini key %q", ErrInvalid, key)
	}
	if alias, ok := keyAliases[trimmed]; ok {
		return alias, nil
	}
	return trimmed, nil
}

func renderSection(section Section) string {
	var out strings.Builder
	out.WriteString("\n\n[" + section.Name + "]\n")
	keys := make([]string, 0, len(section.Values))
	for key := range section.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&out, "%s = %s\n", key, section.Values[key])
	}
	return out.String()
}

// removeSection drops the named section by scanning line by line, so a
// `[name]` appearing inside a comment or value never triggers a false match.
func removeSection(content, name string) string {
	lines := strings.Split(content, "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
			continue
		}
		if start >= 0 {
			end = i
			break
		}
		if strings.TrimSpace(trimmed[1:len(trimmed)-1]) == name {
			start = i
		}
	}
	if start < 0 {
		return content
	}
	for end > start+1 {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed != "" && !strings.HasPrefix(trimmed, ";") && !strings.HasPrefix(trimmed, "#") {
			break
		}
		end--
	}
	return strings.Join(append(lines[:start], lines[end:]...), "\n")
}
