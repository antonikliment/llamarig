## Summary

<!-- What changed and why? -->

## Reuse-first review

- [ ] I searched for existing services/packages before adding logic.
- [ ] I reused or extended the existing owner where possible.
- [ ] I did not duplicate validation, parsing, file-writing, runtime, config, model, HTTP, or MCP logic.
- [ ] Any new package/service is justified below.

Existing code reused:

<!-- e.g. core/serverconfigs, core/control, platform/filedoc -->

New logic justification:

<!-- Required if adding new functions, packages, services, handlers, or managers -->

## Budgets

- [ ] `./custom-golangci-lint run` passes.
- [ ] Implementation Go LOC remains under 10,025 according to the `goclocbudget` linter.
