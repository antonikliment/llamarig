package control

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// helpParamsTTL is how long parsed llama-server --help output is reused before
// re-shelling out. The flag list only changes when the binary is upgraded, so a
// coarse TTL avoids spawning a subprocess on every UI load.
const helpParamsTTL = 10 * time.Minute

// helpParamsCache memoizes GetLlamaServerParams results per executable path.
type helpParamsCache struct {
	mu         sync.Mutex
	executable string
	params     []LlamaServerParam
	expires    time.Time
}

// LlamaServerParam describes a single llama-server CLI flag, parsed from
// the binary's own --help output.
type LlamaServerParam struct {
	Key         string
	Aliases     []string
	ValueHint   string
	Default     string
	Description string
}

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// GetLlamaServerParams shells out to the configured llama-server executable
// with --help and parses the flag list from its output. Callers should treat
// a non-nil error as non-fatal and fall back to a static catalog, since the
// binary may not be installed or may be a version with a different format.
func (m *Manager) GetLlamaServerParams(ctx context.Context) ([]LlamaServerParam, error) {
	executable := m.routerConfigSnapshot(ctx).Executable
	if executable == "" {
		executable = "llama-server"
	}

	m.helpCache.mu.Lock()
	if m.helpCache.executable == executable && time.Now().Before(m.helpCache.expires) {
		params := m.helpCache.params
		m.helpCache.mu.Unlock()
		return params, nil
	}
	m.helpCache.mu.Unlock()

	runner := m.helpRunner
	if runner == nil {
		runner = execCommandRunner{}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := runner.Run(ctx, executable, "--help")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, Errorf(ErrorInvalidInput, "%s not found; cannot read flag help", executable)
		}
		return nil, Errorf(ErrorInvalidInput, "%s --help failed: %v", executable, err)
	}
	params := parseLlamaServerHelp(string(out))
	if len(params) == 0 {
		return nil, Errorf(ErrorInvalidInput, "no flags parsed from %s --help output", executable)
	}

	m.helpCache.mu.Lock()
	m.helpCache.executable = executable
	m.helpCache.params = params
	m.helpCache.expires = time.Now().Add(helpParamsTTL)
	m.helpCache.mu.Unlock()
	return params, nil
}

var (
	flagTokenPattern  = regexp.MustCompile(`-{1,2}[A-Za-z][A-Za-z0-9_-]*`)
	defaultAnnotation = regexp.MustCompile(`\(default:\s*([^)]*)\)`)
)

// parseLlamaServerHelp parses the free-text --help output emitted by
// llama-server (common/arg.cpp) into structured flag descriptions.
func parseLlamaServerHelp(text string) []LlamaServerParam {
	var params []LlamaServerParam
	var current *LlamaServerParam

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "-----") {
			current = nil
			continue
		}

		param, ok := tryParseFlagLine(line)
		if !ok {
			if current != nil {
				appendDescription(current, line)
			}
			continue
		}

		params = append(params, *param)
		current = &params[len(params)-1]
	}

	return params
}

// tryParseFlagLine attempts to parse line as a new flag definition.
// Returns (nil, false) for continuation lines or lines that don't start
// with a flag token at the left margin.
func tryParseFlagLine(line string) (*LlamaServerParam, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, "-") {
		return nil, false
	}
	// Flag definitions are always near the left margin; deeply-indented lines
	// starting with a hyphen are description continuations (e.g. "-ngl is...").
	if indent := len(line) - len(trimmed); indent > 20 {
		return nil, false
	}

	matches := boundedFlagMatches(line)
	if len(matches) == 0 || matches[0][0] != len(line)-len(trimmed) {
		return nil, false
	}

	flagColumnEnd := matches[len(matches)-1][1]
	boundary, valueHint := consumePlaceholder(line, flagColumnEnd)

	keys := parseFlagColumn(line[:flagColumnEnd])
	if len(keys) == 0 {
		return nil, false
	}

	desc := strings.TrimSpace(line[boundary:])
	param := &LlamaServerParam{Key: keys[0], Aliases: keys[1:], ValueHint: valueHint, Description: desc}
	if m := defaultAnnotation.FindStringSubmatch(desc); m != nil {
		param.Default = strings.TrimSpace(m[1])
	}
	return param, true
}

// consumePlaceholder advances boundary past a single-space-separated value
// placeholder token immediately after the last flag (e.g. "N" in "--threads N"),
// returning the new boundary and the placeholder text.
func consumePlaceholder(line string, boundary int) (int, string) {
	if boundary >= len(line) || line[boundary] != ' ' {
		return boundary, ""
	}
	if boundary+1 < len(line) && line[boundary+1] == ' ' {
		return boundary, "" // two or more spaces → description column, not placeholder
	}
	placeholder, end := nextToken(line, boundary+1)
	if placeholder == "" {
		return boundary, ""
	}
	return end, placeholder
}

// boundedFlagMatches finds flag tokens (e.g. "--threads") that begin at
// the start of the line or right after whitespace/comma, so hyphenated
// description words like "memory-map" aren't mistaken for a flag.
func boundedFlagMatches(line string) [][]int {
	all := flagTokenPattern.FindAllStringIndex(line, -1)
	matches := make([][]int, 0, len(all))
	for _, m := range all {
		if m[0] == 0 || line[m[0]-1] == ' ' || line[m[0]-1] == ',' {
			matches = append(matches, m)
		}
	}
	return matches
}

// nextToken returns the run of non-space characters starting at offset,
// along with the index immediately following it.
func nextToken(line string, offset int) (string, int) {
	end := offset
	for end < len(line) && line[end] != ' ' {
		end++
	}
	return line[offset:end], end
}

func appendDescription(param *LlamaServerParam, line string) {
	text := strings.TrimSpace(line)
	if text == "" {
		return
	}
	if param.Description != "" {
		param.Description += " " + text
	} else {
		param.Description = text
	}
	if param.Default == "" {
		if m := defaultAnnotation.FindStringSubmatch(text); m != nil {
			param.Default = strings.TrimSpace(m[1])
		}
	}
}

// parseFlagColumn splits the left column of a flag line, e.g. "-t,
// --threads", into normalized keys with dashes stripped. Long flags are
// ordered first so the canonical key matches the INI convention used
// elsewhere in the preset editor.
func parseFlagColumn(column string) []string {
	var long, short []string
	for _, field := range strings.Split(column, ",") {
		flag := strings.TrimSpace(field)
		if !flagTokenPattern.MatchString(flag) {
			continue
		}
		key := strings.TrimLeft(flag, "-")
		if key == "" {
			continue
		}
		if strings.HasPrefix(flag, "--") {
			long = append(long, key)
		} else {
			short = append(short, key)
		}
	}
	return append(long, short...)
}
