# Spec: replace direct CLI parsing with cobra

Status: implemented · Scope: `cmd/`, `adapters/cli/` · Target branch base: `main`

## Problem

The CLI subcommands (`info`, `status`, `profiles`, `profile`, `start`, `stop`,
`restart`) are registered in `cmd/cli.go` with `DisableFlagParsing: true`. Cobra
is therefore used only as a router: it hands the raw argv (including the command
name) to `cli.Run`, which re-parses `--socket` and `--json` itself with the
stdlib `flag` package (`adapters/cli/cli.go:parse`).

Consequences:

- Two parsers for one CLI. Cobra owns the command tree; stdlib `flag` owns the
  flags — split-brain.
- No per-command `--help`, no flag listing, no shell completion for these
  commands (cobra can't see flags it isn't told about).
- Positional-arg validation is hand-rolled inside handlers
  (`if len(c.args) != 1 { ... }` in `commands.go`).
- stdlib `flag` requires flags *before* positional args; the rest of the binary
  (cobra/pflag) allows interspersed flags. Inconsistent UX.

## Goals

- Cobra parses flags and positional args for the CLI subcommands; remove the
  stdlib `flag` parsing in `adapters/cli`.
- Per-command `--help`/usage and (optionally) completion work for CLI commands.
- Declarative positional-arg validation via `cobra.PositionalArgs`.
- **No change** to observable CLI behavior: same command names, same `--socket`
  / `--json` flags and defaults, same stdout (text and JSON) byte-for-byte.

## Non-goals

- No new commands or flags.
- No change to the control RPC / proto contract.
- No switch away from cobra to another framework (decided separately:
  not worth the churn).
- No change to `serve`, `gateway`, `setup`, `down`, `logs`, `tui` wiring.

## Current architecture (for reference)

- `cmd/cli.go` — loops over `cli.CommandNames`, builds one cobra command per
  name with `DisableFlagParsing: true`, calls `cli.Run(ctx, cli.Options{Args, Env, Out})`.
- `adapters/cli/cli.go` — `Options{Args, Env, Out}`; `parse()` builds a
  `flag.FlagSet`, reads `--socket` (default `env(ProjectSocketEnv)`) and
  `--json`, returns a `command{name, args, socket, json, out}`.
- `adapters/cli/commands.go` — `commandRegistry` (name → handler) is the single
  source of truth; `run()` dispatches; handlers do their own arg-count checks.
- `adapters/cli/output.go` — text/JSON formatting per command.

## Proposed design

Keep `adapters/cli` as the home of handler logic and output formatting, but feed
it **already-parsed** inputs. Cobra (in `cmd/`) owns parsing.

### 1. `adapters/cli` stays cobra-free

Do **not** import cobra into `adapters/cli` (keeps the adapter independent of the
CLI framework and unit-testable in isolation). Express everything cobra needs as
plain data.

Change `Options` from raw argv to parsed fields:

```go
type Options struct {
    Command string    // subcommand name, e.g. "status"
    Args    []string  // positional args (after flag parsing)
    Socket  string
    JSON    bool
    Out     io.Writer
}

func Run(ctx context.Context, opts Options) error // unchanged signature; builds
                                                   // command{} directly from opts
```

- Delete `parse()` and the `flag` import from `adapters/cli/cli.go`.
- `command{}` is constructed directly from `Options` (no FlagSet).
- `Run` validates positional arity from the command registry before dialing, so
  direct adapter callers cannot bypass Cobra and panic a handler.
- `Env` leaves `Options`; the `--socket` env default moves to flag registration
  in `cmd/` (see §3).

Extend the registry so `cmd/` can build cobra commands declaratively, still
without leaking cobra into the adapter:

```go
type CommandSpec struct {
    Name    string
    Usage   string // positional syntax, e.g. "<name>" or "[profile]"
    Short   string
    MinArgs int
    MaxArgs int    // -1 = unbounded
}

func Commands() []CommandSpec // derived from commandRegistry, registration order
```

Per-command help and arg policy (replaces the in-handler checks):

| command  | Usage       | Short                                        | MinArgs | MaxArgs |
|----------|-------------|----------------------------------------------|---------|---------|
| info     |             | Show control daemon information              | 0       | 0       |
| status   |             | Show runtime status                          | 0       | 0       |
| profiles |             | List profiles                                | 0       | 0       |
| profile  | `<name>`    | Show a profile                               | 1       | 1       |
| start    | `[profile]` | Start the requested or default profile       | 0       | 1       |
| stop     | `[profile]` | Stop the requested or all running profiles   | 0       | 1       |
| restart  | `[profile]` | Restart the requested or running profiles    | 0       | 1       |

Handlers (`runProfile`, `runAction`) drop their `if len(c.args) …` guards — cobra
enforces arity before the handler runs.

### 2. `CommandNames` → `Commands()`

`cmd/cli.go` switches from `cli.CommandNames` to `cli.Commands()`. Keep
`CommandNames` only if another caller needs it; otherwise remove it.

### 3. `cmd/cli.go` builds real cobra commands

```go
func cliCommands() []*cobra.Command {
    cmds := make([]*cobra.Command, 0, len(cli.Commands()))
    for _, spec := range cli.Commands() {
        socket, jsonOut := "", false
        c := &cobra.Command{
            Use:   strings.TrimSpace(spec.Name + " " + spec.Usage),
            Short: spec.Short,
            Args:  argsValidator(spec), // ExactArgs / RangeArgs / NoArgs
            ValidArgsFunction: cobra.NoFileCompletions,
            RunE: func(cmd *cobra.Command, args []string) error {
                socketPath := socket
                if !cmd.Flags().Changed("socket") {
                    socketPath = os.Getenv(config.ProjectSocketEnv)
                }
                return cli.Run(cmd.Context(), cli.Options{
                    Command: spec.Name,
                    Args:    args,
                    Socket:  socketPath,
                    JSON:    jsonOut,
                    Out:     cmd.OutOrStdout(),
                })
            },
        }
        c.Flags().StringVar(&socket, "socket", "", config.ProjectDisplayName+" control Unix socket")
        c.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
        cmds = append(cmds, c)
    }
    return cmds
}
```

`argsValidator(spec)` maps `MinArgs/MaxArgs` → `cobra.NoArgs` /
`cobra.ExactArgs(n)` / `cobra.RangeArgs(min, max)` / `cobra.MinimumNArgs(min)`.

Flags are **local per command** (not persistent on root) so `--json` and
`--socket` do not leak onto `serve`/`gateway`/etc., which must not accept them.
Static Cobra completion can now discover the commands and flags. Dynamic
RPC-backed profile-name completion remains out of scope.

## Behavior compatibility

- `llamarig status --json`, `llamarig start foo --socket /path` keep working.
- When `--socket` is omitted, its value is read from `ProjectSocketEnv` at
  command execution time; an explicit flag always wins.
- Output text/JSON identical (output.go untouched).
- Improvements (acceptable, arguably better):
  - Flags may now appear after positional args (pflag default).
  - Unknown command / wrong arg count produce cobra's standard usage errors
    instead of the previous ad-hoc `fmt.Errorf`. Snapshot/adjust any test that
    asserts on the old message text.
  - `--help` uses Cobra's successful help path instead of returning
    `flag.ErrHelp`.
- The undocumented stdlib spellings `-json` and `-socket` are not preserved;
  supported flags use their documented `--json` and `--socket` forms.

## Test plan

`cmd/` and `adapters/cli` have no e2e coverage today; this migration must not
reduce coverage and should add a little:

1. `adapters/cli/cli_test.go` — pass `Command/Socket/JSON` directly and assert
   exact JSON and text output for every command through the existing fake Unix
   socket ControlService pattern. Cover registry order and defensive validation.
2. New `cmd/cli_test.go` — build the root command via `NewRootCommand()`,
   `SetArgs([]string{"status", "--json"})`, `SetOut(buf)`, `Execute()`, and
   assert: flag parsing reaches the handler, arg-count validation rejects bad
   input (e.g. `profile` with 0 args), and `--help` lists the flags. This gives
   the binary entrypoint real in-process coverage it currently lacks.
3. Run project gates with `make test` and `make lint`.

## Risks

- Low. Surface is ~7 commands and 2 flags; logic and output formatting are
  untouched. Main risk is a test asserting on the old error-message text — fix
  those snapshots.
- Cross-package coupling stays clean: cobra is confined to `cmd/`; `adapters/cli`
  exposes only plain `CommandSpec` data.

## LoC impact (estimate)

Roughly neutral: `adapters/cli` loses `parse()` + the `flag` import and the
in-handler arg checks (~−15); `cmd/cli.go` gains flag registration + an
`argsValidator` helper (~+15). The win is correctness/UX (per-command help,
declarative arg validation, single parser), not line count.

## Rollout

Implemented on the existing feature branch without changing the control RPC,
generated protobuf output, or unrelated command wiring.
