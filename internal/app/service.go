package app

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/mrtuuro/go-switcher/internal/install"
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
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return "", err
	}

	all, err := s.ReleaseClient.Fetch(ctx)
	if err != nil {
		return "", err
	}

	archive, normalized, err := releases.FindArchive(all, normalized, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if err := install.InstallGoArchive(ctx, s.Paths, normalized, archive); err != nil {
		return "", err
	}

	if err := switcher.EnsureShims(s.Paths); err != nil {
		return "", err
	}

	return normalized, nil
}

func (s *Service) Use(ctx context.Context, version string, scope switcher.Scope, cwd string) (string, string, error) {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return "", "", err
	}

	if !switcher.ToolchainExists(s.Paths, normalized) {
		if _, err := s.Install(ctx, normalized); err != nil {
			return "", "", fmt.Errorf("install %s before switching: %w", normalized, err)
		}
	}

	if err := switcher.SetActiveVersion(normalized, scope, cwd, s.Paths); err != nil {
		return "", "", err
	}

	if err := switcher.EnsureShims(s.Paths); err != nil {
		return "", "", err
	}

	lintVersion, err := s.SyncToolsForVersion(ctx, normalized)
	if err != nil {
		return "", "", err
	}

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
	cfg, err := switcher.ReadConfig(s.Paths)
	if err != nil {
		return "", err
	}

	lintVersion, err := tools.EnsureForGoVersion(ctx, s.Paths, &cfg, goVersion)
	if err != nil {
		return "", err
	}

	if err := switcher.WriteConfig(s.Paths, cfg); err != nil {
		return "", err
	}

	return lintVersion, nil
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
