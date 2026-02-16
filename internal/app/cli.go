package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mrtuuro/go-switcher/internal/switcher"
	"github.com/mrtuuro/go-switcher/internal/tui"
)

type CLI struct {
	stdout  io.Writer
	stderr  io.Writer
	cwd     string
	service *Service
}

func NewCLI(stdout io.Writer, stderr io.Writer, cwd string) (*CLI, error) {
	service, err := NewService()
	if err != nil {
		return nil, err
	}
	return &CLI{
		stdout:  stdout,
		stderr:  stderr,
		cwd:     cwd,
		service: service,
	}, nil
}

func (c *CLI) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		c.printUsage()
		return nil
	}

	switch args[0] {
	case "help", "--help", "-h":
		c.printUsage()
		return nil
	case "current":
		return c.runCurrent()
	case "list":
		return c.runList(ctx, args[1:])
	case "install":
		return c.runInstall(ctx, args[1:])
	case "use":
		return c.runUse(ctx, args[1:])
	case "tools":
		return c.runTools(ctx, args[1:])
	case "exec":
		return c.runExec(ctx, args[1:])
	case "tui":
		return tui.Run(ctx, c.service, c.cwd)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (c *CLI) runCurrent() error {
	active, err := c.service.Current(c.cwd)
	if err != nil {
		if err == switcher.ErrNoActiveVersion {
			c.println("no active Go version configured")
			return nil
		}
		return err
	}

	c.printf("%s (%s)\n", active.Version, active.Scope)
	c.printf("source: %s\n", active.Source)
	return nil
}

func (c *CLI) runList(ctx context.Context, args []string) error {
	remote := false
	for _, arg := range args {
		if arg == "--remote" {
			remote = true
			continue
		}
		return fmt.Errorf("unknown list argument %q", arg)
	}

	if remote {
		versions, err := c.service.ListRemote(ctx)
		if err != nil {
			return err
		}
		if len(versions) == 0 {
			c.println("no remote versions found for this platform")
			return nil
		}
		for _, version := range versions {
			c.println(version)
		}
		return nil
	}

	localVersions, err := c.service.ListLocal()
	if err != nil {
		return err
	}

	active, err := c.service.Current(c.cwd)
	if err != nil && err != switcher.ErrNoActiveVersion {
		return err
	}

	if len(localVersions) == 0 {
		c.println("no local toolchains installed")
		return nil
	}

	for _, version := range localVersions {
		prefix := "  "
		if err == nil && version == active.Version {
			prefix = "* "
		}
		c.printf("%s%s\n", prefix, version)
	}

	return nil
}

func (c *CLI) runInstall(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: switcher install <go-version>")
	}

	version, err := c.service.Install(ctx, args[0])
	if err != nil {
		return err
	}

	c.printf("installed %s\n", version)
	pathHint, inPath, err := c.service.PathHint()
	if err == nil && !inPath {
		c.printf("add %s to PATH to use shims\n", pathHint)
	}
	return nil
}

func (c *CLI) runUse(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: switcher use <go-version> [--scope global|local]")
	}

	version := ""
	scope := switcher.ScopeGlobal
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--scope="):
			rawScope := strings.TrimPrefix(arg, "--scope=")
			parsed, err := switcher.ParseScope(rawScope)
			if err != nil {
				return err
			}
			scope = parsed
		case arg == "--scope":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --scope")
			}
			parsed, err := switcher.ParseScope(args[i+1])
			if err != nil {
				return err
			}
			scope = parsed
			i++
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown flag %q", arg)
		default:
			if version != "" {
				return fmt.Errorf("multiple versions provided")
			}
			version = arg
		}
	}

	if version == "" {
		return fmt.Errorf("missing go version")
	}

	resolvedVersion, lintVersion, err := c.service.Use(ctx, version, scope, c.cwd)
	if err != nil {
		return err
	}

	c.printf("configured Go version %s (%s)\n", resolvedVersion, scope)
	active, activeErr := c.service.Current(c.cwd)
	if activeErr == nil {
		if active.Version == resolvedVersion && active.Scope == scope {
			c.printf("effective active version is %s (%s)\n", active.Version, active.Scope)
		} else {
			c.printf("effective active version is %s (%s)\n", active.Version, active.Scope)
			c.println("note: local scope overrides global in this directory")
		}
	}
	c.printf("golangci-lint synced to %s\n", lintVersion)
	pathHint, inPath, err := c.service.PathHint()
	if err == nil && !inPath {
		c.printf("add %s to PATH to use shims\n", pathHint)
	}
	return nil
}

func (c *CLI) runTools(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: switcher tools sync [--scope global|local]")
	}

	if args[0] != "sync" {
		return fmt.Errorf("unknown tools command %q", args[0])
	}

	scopeOverride := ""
	flags := args[1:]
	for i := 0; i < len(flags); i++ {
		arg := flags[i]
		switch {
		case strings.HasPrefix(arg, "--scope="):
			scopeOverride = strings.TrimPrefix(arg, "--scope=")
		case arg == "--scope":
			if i+1 >= len(flags) {
				return fmt.Errorf("missing value for --scope")
			}
			scopeOverride = flags[i+1]
			i++
		default:
			return fmt.Errorf("unknown tools sync flag %q", arg)
		}
	}

	goVersion, lintVersion, err := c.service.SyncTools(ctx, c.cwd, scopeOverride)
	if err != nil {
		return err
	}

	c.printf("synced golangci-lint %s for %s\n", lintVersion, goVersion)
	return nil
}

func (c *CLI) runExec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: switcher exec <tool> [args...]")
	}

	tool := args[0]
	binaryPath, activeVersion, err := c.service.ResolveBinaryForTool(c.cwd, tool)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, binaryPath, args[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("run %s with %s: %w", tool, activeVersion, runErr)
	}

	return nil
}

func (c *CLI) printUsage() {
	usage := `switcher - Go toolchain switcher

Usage:
  switcher current
  switcher list [--remote]
  switcher install <go-version>
  switcher use <go-version> [--scope global|local]
  switcher tools sync [--scope global|local]
  switcher tui

Notes:
  - local scope uses .switcher-version in the working tree
  - local scope overrides global scope when both are set
  - add ~/.switcher/bin to PATH to use go/gofmt/golangci-lint shims
`
	c.println(usage)
}

func (c *CLI) println(line string) {
	_, _ = fmt.Fprintln(c.stdout, line)
}

func (c *CLI) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(c.stdout, format, args...)
}
