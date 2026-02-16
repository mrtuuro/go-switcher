package app

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/mrtuuro/go-switcher/internal/install"
	"github.com/mrtuuro/go-switcher/internal/progress"
	"github.com/mrtuuro/go-switcher/internal/releases"
	"github.com/mrtuuro/go-switcher/internal/switcher"
	"github.com/mrtuuro/go-switcher/internal/tools"
	"github.com/mrtuuro/go-switcher/internal/versionutil"
)

type Service struct {
	Paths         switcher.Paths
	ReleaseClient *releases.Client
}

func NewService() (*Service, error) {
	paths, err := switcher.DefaultPaths()
	if err != nil {
		return nil, err
	}

	service := &Service{
		Paths:         paths,
		ReleaseClient: releases.NewClient(),
	}

	if err := switcher.EnsureLayout(paths); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *Service) ListLocal() ([]string, error) {
	return switcher.ListInstalledVersions(s.Paths)
}

func (s *Service) ListRemote(ctx context.Context) ([]string, error) {
	all, err := s.ReleaseClient.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return releases.AvailableVersions(all, runtime.GOOS, runtime.GOARCH), nil
}

func (s *Service) Current(cwd string) (switcher.ActiveVersion, error) {
	return switcher.ResolveActiveVersion(cwd, s.Paths)
}

func (s *Service) Install(ctx context.Context, version string) (string, error) {
	return s.InstallWithProgress(ctx, version, nil)
}

func (s *Service) InstallWithProgress(ctx context.Context, version string, reporter progress.Reporter) (string, error) {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return "", err
	}

	progress.Emit(reporter, "release-fetch", "Fetching Go release metadata...", 0, 0)
	all, err := s.ReleaseClient.Fetch(ctx)
	if err != nil {
		return "", err
	}

	progress.Emit(reporter, "release-select", fmt.Sprintf("Selecting %s for %s/%s", normalized, runtime.GOOS, runtime.GOARCH), 0, 0)
	archive, normalized, err := releases.FindArchive(all, normalized, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if err := install.InstallGoArchiveWithOptions(ctx, s.Paths, normalized, archive, install.InstallOptions{Reporter: reporter}); err != nil {
		return "", err
	}

	progress.Emit(reporter, "shim-update", "Updating tool shims...", 0, 0)
	if err := switcher.EnsureShims(s.Paths); err != nil {
		return "", err
	}

	progress.Emit(reporter, "go-install", fmt.Sprintf("Ready: %s", normalized), 0, 0)
	return normalized, nil
}

func (s *Service) Use(ctx context.Context, version string, scope switcher.Scope, cwd string) (string, string, error) {
	return s.UseWithProgress(ctx, version, scope, cwd, nil)
}

func (s *Service) UseWithProgress(ctx context.Context, version string, scope switcher.Scope, cwd string, reporter progress.Reporter) (string, string, error) {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return "", "", err
	}

	if !switcher.ToolchainExists(s.Paths, normalized) {
		progress.Emit(reporter, "go-install", fmt.Sprintf("%s is not installed yet", normalized), 0, 0)
		if _, err := s.InstallWithProgress(ctx, normalized, reporter); err != nil {
			return "", "", fmt.Errorf("install %s before switching: %w", normalized, err)
		}
	} else {
		progress.Emit(reporter, "go-install", fmt.Sprintf("Using installed toolchain %s", normalized), 0, 0)
	}

	progress.Emit(reporter, "scope-update", fmt.Sprintf("Applying %s scope...", scope), 0, 0)
	if err := switcher.SetActiveVersion(normalized, scope, cwd, s.Paths); err != nil {
		return "", "", err
	}

	progress.Emit(reporter, "shim-update", "Refreshing shims...", 0, 0)
	if err := switcher.EnsureShims(s.Paths); err != nil {
		return "", "", err
	}

	progress.Emit(reporter, "lint-sync", "Syncing golangci-lint...", 0, 0)
	lintVersion, err := s.SyncToolsForVersionWithProgress(ctx, normalized, reporter)
	if err != nil {
		return "", "", err
	}
	progress.Emit(reporter, "done", fmt.Sprintf("Switch complete: %s (%s)", normalized, scope), 0, 0)

	return normalized, lintVersion, nil
}

func (s *Service) SyncTools(ctx context.Context, cwd string, scopeOverride string) (string, string, error) {
	var (
		activeVersion string
		err           error
	)

	if scopeOverride == "" {
		resolved, resolveErr := switcher.ResolveActiveVersion(cwd, s.Paths)
		if resolveErr != nil {
			return "", "", resolveErr
		}
		activeVersion = resolved.Version
	} else {
		scope, parseErr := switcher.ParseScope(scopeOverride)
		if parseErr != nil {
			return "", "", parseErr
		}

		switch scope {
		case switcher.ScopeLocal:
			localVersion, _, found, localErr := switcher.FindLocalVersion(cwd)
			if localErr != nil {
				return "", "", localErr
			}
			if !found {
				return "", "", fmt.Errorf("no local .switcher-version found")
			}
			activeVersion = localVersion
		case switcher.ScopeGlobal:
			globalVersion, found, globalErr := switcher.GlobalVersion(s.Paths)
			if globalErr != nil {
				return "", "", globalErr
			}
			if !found {
				return "", "", fmt.Errorf("no global version configured")
			}
			activeVersion = globalVersion
		}
	}

	lintVersion, err := s.SyncToolsForVersion(ctx, activeVersion)
	if err != nil {
		return "", "", err
	}

	return activeVersion, lintVersion, nil
}

func (s *Service) SyncToolsForVersion(ctx context.Context, goVersion string) (string, error) {
	return s.SyncToolsForVersionWithProgress(ctx, goVersion, nil)
}

func (s *Service) SyncToolsForVersionWithProgress(ctx context.Context, goVersion string, reporter progress.Reporter) (string, error) {
	cfg, err := switcher.ReadConfig(s.Paths)
	if err != nil {
		return "", err
	}

	lintVersion, err := tools.EnsureForGoVersionWithOptions(ctx, s.Paths, &cfg, goVersion, tools.EnsureOptions{Reporter: reporter})
	if err != nil {
		return "", err
	}

	if err := switcher.WriteConfig(s.Paths, cfg); err != nil {
		return "", err
	}

	return lintVersion, nil
}

func (s *Service) DeleteInstalledWithProgress(ctx context.Context, cwd string, version string, reporter progress.Reporter) (switcher.DeleteResult, error) {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return switcher.DeleteResult{}, err
	}

	progress.Emit(reporter, "delete", fmt.Sprintf("Removing toolchain %s...", normalized), 0, 0)

	active, activeErr := s.Current(cwd)
	hasActive := activeErr == nil
	if activeErr != nil && activeErr != switcher.ErrNoActiveVersion {
		return switcher.DeleteResult{}, activeErr
	}

	if err := switcher.DeleteInstalledVersion(s.Paths, normalized); err != nil {
		return switcher.DeleteResult{}, err
	}

	if err := s.deleteLintMapping(normalized); err != nil {
		return switcher.DeleteResult{}, err
	}

	result := switcher.DeleteResult{DeletedVersion: normalized}
	if !hasActive || active.Version != normalized {
		current, err := s.Current(cwd)
		if err == nil {
			result.ActiveAfter = current
		}
		progress.Emit(reporter, "delete", fmt.Sprintf("Deleted %s", normalized), 0, 0)
		return result, nil
	}

	result.WasActive = true

	remaining, err := s.ListLocal()
	if err != nil {
		return switcher.DeleteResult{}, err
	}

	if len(remaining) == 0 {
		progress.Emit(reporter, "delete", "Deleted active version; no installed versions remain", 0, 0)
		if active.Scope == switcher.ScopeLocal {
			if err := switcher.ClearLocalVersionAtPath(active.Source); err != nil {
				return switcher.DeleteResult{}, err
			}
		} else {
			if err := switcher.ClearGlobalVersion(s.Paths); err != nil {
				return switcher.DeleteResult{}, err
			}
		}

		return result, nil
	}

	newest := remaining[0]
	result.SwitchedToNewest = true
	progress.Emit(reporter, "switch", fmt.Sprintf("Deleted active version; switching to newest %s", newest), 0, 0)

	if active.Scope == switcher.ScopeLocal {
		if err := switcher.SetLocalVersionAtPath(active.Source, newest); err != nil {
			return switcher.DeleteResult{}, err
		}
	} else {
		if err := switcher.SetGlobalVersion(s.Paths, newest); err != nil {
			return switcher.DeleteResult{}, err
		}
	}

	progress.Emit(reporter, "shim-update", "Refreshing shims...", 0, 0)
	if err := switcher.EnsureShims(s.Paths); err != nil {
		return switcher.DeleteResult{}, err
	}

	progress.Emit(reporter, "lint-sync", "Syncing golangci-lint for new active version...", 0, 0)
	if _, err := s.SyncToolsForVersionWithProgress(ctx, newest, reporter); err != nil {
		result.ToolSyncWarning = err.Error()
		progress.Emit(reporter, "lint-sync", fmt.Sprintf("Warning: %s", err.Error()), 0, 0)
	}

	current, err := s.Current(cwd)
	if err == nil {
		result.ActiveAfter = current
	}

	progress.Emit(reporter, "delete", fmt.Sprintf("Deleted %s", normalized), 0, 0)
	return result, nil
}

func (s *Service) deleteLintMapping(goVersion string) error {
	cfg, err := switcher.ReadConfig(s.Paths)
	if err != nil {
		return err
	}

	if cfg.GolangCILintByGo == nil {
		return nil
	}

	if _, ok := cfg.GolangCILintByGo[goVersion]; !ok {
		return nil
	}

	delete(cfg.GolangCILintByGo, goVersion)
	return switcher.WriteConfig(s.Paths, cfg)
}

func (s *Service) ResolveBinaryForTool(cwd string, tool string) (string, string, error) {
	active, err := switcher.ResolveActiveVersion(cwd, s.Paths)
	if err != nil {
		return "", "", err
	}

	switch tool {
	case "go", "gofmt":
		binary, err := switcher.GoToolBinary(s.Paths, active.Version, tool)
		if err != nil {
			return "", "", err
		}
		return binary, active.Version, nil
	case "golangci-lint":
		cfg, err := switcher.ReadConfig(s.Paths)
		if err != nil {
			return "", "", err
		}
		binary, _, err := tools.ResolveBinary(s.Paths, cfg, active.Version)
		if err != nil {
			return "", "", err
		}
		return binary, active.Version, nil
	default:
		return "", "", fmt.Errorf("unsupported tool %q", tool)
	}
}

func (s *Service) EnsureShims() error {
	return switcher.EnsureShims(s.Paths)
}

func (s *Service) PathHint() (string, bool, error) {
	return switcher.EnsurePathHint(s.Paths)
}

func CurrentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
