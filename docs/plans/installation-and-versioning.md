# Installation and Versioned Distribution

## Summary

Deliver installation improvements in three gated phases:

1. Native signed binaries, embedded web UI, zero-config startup, `llamarig doctor`, and optional pinned CPU/Metal llama-server bundles.
2. Homebrew, WinGet, `.deb`, and `.rpm` distribution.
3. CPU, CUDA, Vulkan, and ROCm all-in-one container images.

Target stable SemVer release `v0.1.0`. Foundation releases use prerelease tags and support Linux and macOS on amd64/arm64. Windows joins only after its local-control and process lifecycle are portable. Keep production Go below 10,000 gocloc lines by replacing the large setup wizard with zero-config initialization and consolidating platform process logic.

## Delivered Foundation

- A manual GitHub Actions workflow runs only from `main`, accepts one prerelease SemVer input, and publishes a GitHub prerelease after all gates pass.
- Release metadata has one owner and feeds `llamarig version`, `llamarig --version`, JSON output, RPC/REST build info, and MCP metadata. Untagged builds report `dev`.
- Release assembly builds `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` archives containing the embedded web UI, README, and MIT license, plus `SHA256SUMS`.
- `.woodpecker/release.yml.example` is an inactive fallback that reuses the same package script. GitHub Actions remains the release owner.
- Unsigned stable releases, Windows archives, signing, provenance, notices, llama.cpp bundles, package channels, doctor, zero-config setup, and containers remain gated roadmap work.

## Implementation Changes

### Phase 1 — Native Release Completion

- Adopt the MIT license as `Copyright (c) 2026 LlamaRig contributors`.
- Generate `THIRD_PARTY_NOTICES` from Go, pnpm, and bundled llama.cpp dependencies; fail releases when required license data is missing.
- Add shared build metadata populated from Git tag, commit, and commit timestamp:
  - `llamarig version`
  - `llamarig --version`
  - `llamarig version --json`
  - Populate the RPC `BuildInfo` field and REST `/info` build object.
  - Use the same version for MCP metadata.
  - Untagged builds report `dev`.
- Build the frontend with the frozen pnpm lockfile before Go compilation. Continue embedding `webui/dist` through existing `webui.Files`; never ship separate web assets.
- Replace the interactive first-run wizard with idempotent initialization:
  - Create localhost-only default config, models directory, and `models.ini`.
  - Default `router.executable` to `auto`.
  - Keep `llamarig setup` as an explicit, non-destructive initializer and path reporter.
  - Advanced configuration remains available through YAML and the existing web configuration UI.
- Centralize llama-server resolution under existing runtime ownership:
  1. `LLAMARIG_LLAMA_SERVER`
  2. Explicit non-`auto` `router.executable`
  3. Executable-relative bundled `runtime/llama-server[.exe]`
  4. `llama-server` from `PATH`
- Preserve compatibility with existing configs containing `router.executable: llama-server`.

### `llamarig doctor`

Add read-only diagnostics with human and stable JSON output:

```text
llamarig doctor
llamarig doctor --json
```

JSON uses:

```json
{
  "schema_version": 1,
  "version": "v0.1.0",
  "checks": [
    {
      "id": "runtime.executable",
      "status": "pass|warn|fail",
      "summary": "...",
      "details": {},
      "remediation": "..."
    }
  ],
  "counts": {"pass": 0, "warn": 0, "fail": 0}
}
```

Checks cover:

- Build version, OS, architecture, and configured backend.
- Config validity and unsafe non-loopback exposure.
- llama-server resolution, executable permissions, version, expected pinned tag, and `--list-devices`.
- NVIDIA, Vulkan, ROCm, and Metal availability using existing system tools where present.
- Gateway and router ports; an existing healthy LlamaRig service passes, unrelated occupation fails.
- LlamaRig home, config, models, cache, logs, and runtime directory writability using temporary files that are immediately removed.
- Missing optional GPU drivers warn when CPU remains usable; missing selected backend or llama-server fails.
- Exit `0` with passes or warnings and `1` when any blocking check fails.
- No `--fix` behavior.

Reuse existing config path resolution, signal collection, runtime configuration, and executable probing. Do not duplicate setup validation.

### Windows Portability

- Introduce build-tagged local-control transport:
  - Unix socket on Linux and macOS, unchanged.
  - Current-user-restricted named pipe on Windows using the already-present `go-winio` dependency.
  - Retain `LLAMARIG_CONTROL_SOCKET` and `--socket` as endpoint overrides.
- Move process start, liveness, graceful termination, forced termination, and detached-process flags behind OS-specific functions.
- Unix retains process-group signals.
- Windows uses a Job Object for newly launched llama-server processes and process termination for recovered PID-file processes.
- Preserve PID recovery and shutdown timeouts on every platform.
- Use one receiver style consistently for every touched Go type.

### Native Artifacts and Bundles

The delivered unsigned prerelease foundation builds four targets:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`

Complete Phase 1 by adding signed builds for all six targets:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`
- `windows/amd64`, `windows/arm64`

Publish:

- `llamarig_<version>_<os>_<arch>.tar.gz`, or `.zip` on Windows.
- `llamarig_<version>_<os>_<arch>_bundle.*` containing the same LlamaRig binary plus normalized `runtime/` contents from a pinned upstream llama.cpp CPU build; macOS uses its Metal-capable build.
- SHA-256 checksums, a detached checksum signature, and GitHub build provenance.
- Apple Developer ID signing and notarization for LlamaRig and redistributed runtime binaries and libraries.
- Authenticode signing for Windows executables and redistributed DLLs.
- Bundles retain the upstream llama.cpp license and source/tag information.

Store one reviewed llama.cpp tag plus source asset SHA-256 hashes or digests in release configuration. Dependency-update PRs may change it only after bundle and live-runtime tests pass. Never resolve `latest` during publishing.

### Phase 2 — Package Channels

Use one GoReleaser workflow where supported, with small validation scripts around external repositories.

- Homebrew formula in `antonikliment/homebrew-tap`:
  - `brew install antonikliment/tap/llamarig`
  - Install signed LlamaRig-only archives; llama-server remains optional.
- WinGet package `AntonKliment.LlamaRig`:
  - Use signed Windows portable archives.
  - Stable releases automatically open or update a manifest PR in `microsoft/winget-pkgs`.
- Attach `.deb` and `.rpm` packages for amd64 and arm64 to GitHub releases:
  - Install `/usr/bin/llamarig` plus license and notices.
  - Do not install system services or modify user configuration.
  - Do not operate apt or yum repositories in this phase.
- Stable tags publish external package channels. Prerelease tags publish GitHub artifacts only.

### Phase 3 — Container Images

Publish to `ghcr.io/antonikliment/llamarig`:

- `<version>-cpu` and `cpu`
- `<version>-cuda` and `cuda`
- `<version>-vulkan` and `vulkan`
- `<version>-rocm` and `rocm`
- `latest`, aliasing stable CPU only

Architecture matrix follows upstream availability:

- CPU, CUDA, Vulkan: linux/amd64 and linux/arm64.
- ROCm: linux/amd64.

Build from pinned upstream `llama.cpp:server*` images by digest.

Container behavior:

- One invocation starts the control daemon and web gateway through a small signal-forwarding entrypoint; preserve the existing two-service architecture and avoid adding a third combined Go command.
- Exit if either service unexpectedly terminates; stop and reap both services on SIGTERM or SIGINT.
- Persist `LLAMARIG_HOME=/data`; models live under `/data/models`.
- Listen on `0.0.0.0:7000` inside the container while documentation publishes it to host loopback by default.
- Expose only port `7000`; llama-server remains container-local.
- Run as a non-root LlamaRig user and document required GPU device and group flags.
- Include a health check against `/health`.
- Sign images keylessly and attach GitHub provenance attestations.

## CI, Testing, and Release Gates

- Required repository checks:
  - `make test`
  - `make lint`
  - `pnpm run verify:web`
  - Production Go LOC below 10,000, with a target buffer at or below 9,950.
- Run native Go tests on Linux, macOS, and Windows runners for both architectures where runners or emulation are available; cross-build the four delivered targets now and all six after Windows portability lands.
- Validate clean-checkout frontend embedding on pull requests; add GoReleaser snapshots with package-channel work.
- Test version injection in CLI, JSON, RPC `/info`, and MCP metadata.
- Test first-run initialization, idempotence, old-config compatibility, bundled/PATH/environment runtime precedence, and missing-runtime errors.
- Test doctor pass/warn/fail classification, JSON schema, occupied ports, permissions, malformed config, and absent drivers.
- Test Unix socket permissions and Windows named-pipe access isolation.
- Extract every archive and package and execute `llamarig version`; run `llama-server --version` for every bundle.
- Verify macOS signatures and notarization, Windows Authenticode signatures, checksums, and provenance before stable publication.
- Smoke-test the Homebrew formula, WinGet validation, `.deb` installation and removal, and `.rpm` installation and removal without touching user config.
- Start the CPU container and verify `/health`, `/info`, clean shutdown, persistent config, and embedded UI.
- GPU image CI verifies backend binaries and metadata without claiming hardware execution. Add optional self-hosted GPU smoke tests before promoting backend aliases.
- Trigger releases manually through GitHub Actions on `main`; CI validates the requested version, creates the tag, and publishes only after tests, lint, web verification, packaging, smoke checks, and checksums pass.
- Until platform signing lands, accept prerelease tags only. `v0.1.0` remains the first signed stable distribution tag.
- Never commit generated protobuf output under `core/rpc/gen/`.

## Reuse and LOC Budget

- Existing owners:
  - `webui` owns embedded frontend assets.
  - `core/runtime` owns llama-server discovery and lifecycle.
  - `core/setup` owns default files.
  - `platform/process`, `platform/pidfile`, and `core/rpc` own OS portability.
  - `core/signals` supplies reusable hardware probes.
- New logic is limited to release metadata, diagnostics, Windows adapters, and packaging automation.
- Remove the Bubble Tea setup model and prompt-specific rendering and validation. Reduce `core/setup` to safe-default generation and idempotent file creation.
- Consolidate executable resolution shared by setup, doctor, runtime, and live tests.
- Keep release assembly, licensing, and container orchestration in CI or scripts rather than production Go.
- Do not add auto-update, installer curl scripts, background system services, GUI installers, apt or yum repositories, or a new combined service architecture.

## Alternatives Recorded

- Rejected single-launch delivery: too many signing, portability, package-review, and GPU variables for one safe gate.
- Rejected pretending cross-compiled Windows binaries are supported: ship four Unix targets first, then add both Windows architectures with named-pipe and process portability.
- Rejected Compose-only and control-only containers: one-command web experience is primary.
- Rejected private or public-binary-only distribution: chose public OSS under MIT.
- Rejected checksum-only or delayed signing: stable releases require platform signing.
- Rejected all native GPU bundles: CPU and Metal native bundles first; GPU variants use containers.
- Rejected Scoop and dual Windows channels: WinGet provides one mainstream channel.
- Rejected hosted apt and yum repositories: signed release assets avoid repository operations initially.
- Rejected retained or shortened wizard: zero-config startup improves installation and funds the LOC budget.
- Rejected calendar versions: SemVer tags drive all artifacts.
- Rejected latest-at-release and independently versioned llama bundles: a reviewed pinned upstream tag keeps builds reproducible.
- Rejected Homebrew Cask: LlamaRig is primarily a CLI binary, and the requested channel is a formula.
- Rejected an internal `llamarig all` command: the container entrypoint can supervise existing services without weakening service separation.

## Assumptions

- The repository becomes public before package-manager submission.
- Apple Developer ID, App Store Connect, and Windows code-signing credentials will be supplied as protected CI secrets.
- A separate public `antonikliment/homebrew-tap` repository and package-publication token will exist.
- GitHub Container Registry and Actions attestations will be enabled after the repository is public or has a plan supporting private-repository attestations.
- `v0.1.0` is not published until every Phase 1 gate passes; later phases may follow without blocking native binary availability.
