# switcher

`switcher` is a Go toolchain switcher for macOS and Linux.
It installs official Go releases, supports global and project-local version selection, and keeps companion tooling in sync.

## Features

- Install official Go releases for your platform.
- Switch active Go version with `global` or `local` scope.
- Use `.switcher-version` for project-local overrides.
- Sync `golangci-lint` to a compatible version for the selected Go toolchain.
- Launch an interactive terminal UI built with Charm libraries.

## Demo

Quick demo (auto-playing GIF inside the repository):

![go-switcher demo](./assets/go-switcher-demo.gif)

## Installation

### Option 1: Install without Go (release script)

```bash
curl -fsSL https://raw.githubusercontent.com/mrtuuro/go-switcher/main/scripts/install.sh | sh
```

This downloads the latest release binary for your OS/arch and installs it to
`~/.switcher/bin/switcher`.

Pin a specific version if needed:

```bash
curl -fsSL https://raw.githubusercontent.com/mrtuuro/go-switcher/main/scripts/install.sh | SWITCHER_VERSION=v0.1.0 sh
```

### Option 2: Install with `go install`

```bash
go install github.com/mrtuuro/go-switcher/cmd/switcher@latest
```

### Option 3: Download a release binary manually

Download the archive for your OS/arch from GitHub Releases, then place the
`switcher` binary in a directory on your `PATH`.

### Option 4: Build from source

Building from source requires Go 1.23+ because of Charm dependencies.
`make` targets use `GOTOOLCHAIN=auto`, so Go 1.21+ can auto-download a newer
toolchain when allowed.

```bash
make build
```

This produces `./bin/switcher`.

Install as a terminal command and bootstrap shims:

```bash
make install
```

Or from a cloned repo without building:

```bash
make bootstrap
```

Then add the switcher shim directory to PATH:

```bash
export PATH="$HOME/.switcher/bin:$PATH"
```

The shims route `go`, `gofmt`, and `golangci-lint` through `switcher exec ...`.
After changing PATH, restart your shell or run `hash -r`.

### Zsh/Bash setup

```bash
echo 'export PATH="$HOME/.switcher/bin:$PATH"' >> ~/.zshrc
exec zsh
```

## Commands

```bash
switcher current
switcher list
switcher list --remote
switcher install 1.25.0
switcher use 1.25.0 --scope global
switcher use 1.24.3 --scope local
switcher tools sync
switcher tools sync --scope local
switcher tui
```

### TUI controls

- `Tab`: switch between local and remote lists
- `/`: start version search filter
- `Esc`: clear search filter
- `Enter`: use selected version
- `i`: install selected remote version
- `X`: delete selected local installed version
- `r`: refresh current list information
- `s`: toggle scope (`global`/`local`)
- `q`: quit

If you delete the currently active installed version, switcher automatically
sets the active version to the newest remaining installed one.

## Scope resolution

- `local` scope writes `.switcher-version` in the current project.
- `global` scope writes `~/.switcher/config.json`.
- At runtime, local scope always overrides global scope.

## Managed filesystem layout

```text
~/.switcher/
  bin/            # shims (go, gofmt, golangci-lint)
  cache/          # downloaded archives
  config.json     # global settings
  toolchains/     # Go installs (go1.xx.x)
  tools/          # companion tools (golangci-lint)
```

## Development

```bash
make fmt
make lint
make test
make test-one PKG=./internal/switcher TEST='^TestResolveActiveVersion_LocalTakesPrecedence$'
```

## Publishing Releases

- Push commits to `main`.
- Create and push a semver tag (for example `v0.1.0`).
- GitHub Actions publishes release archives for macOS and Linux.

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Notes

- `switcher` currently targets macOS and Linux archives from `go.dev/dl`.
- If `golangci-lint` is missing for the active Go version, run `switcher tools sync`.
- If your active Go is old and source build fails, install from release script instead.
