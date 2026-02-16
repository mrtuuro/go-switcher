package switcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrtuuro/go-switcher/internal/versionutil"
)

const LocalVersionFile = ".switcher-version"

var ErrNoActiveVersion = errors.New("no active go version configured")

type Scope string

const (
	ScopeGlobal Scope = "global"
	ScopeLocal  Scope = "local"
)

func ParseScope(raw string) (Scope, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	switch trimmed {
	case "", string(ScopeGlobal):
		return ScopeGlobal, nil
	case string(ScopeLocal):
		return ScopeLocal, nil
	default:
		return "", fmt.Errorf("invalid scope %q", raw)
	}
}

type ActiveVersion struct {
	Version string
	Scope   Scope
	Source  string
}

func FindLocalVersion(start string) (version string, path string, found bool, err error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve absolute path from %s: %w", start, err)
	}

	info, err := os.Stat(abs)
	if err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	current := abs
	for {
		candidate := filepath.Join(current, LocalVersionFile)
		raw, err := os.ReadFile(candidate)
		if err == nil {
			normalized, normErr := versionutil.NormalizeGoVersion(strings.TrimSpace(string(raw)))
			if normErr != nil {
				return "", "", false, fmt.Errorf("invalid local version in %s: %w", candidate, normErr)
			}
			return normalized, candidate, true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", "", false, fmt.Errorf("read local version file %s: %w", candidate, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", "", false, nil
}

func ResolveActiveVersion(cwd string, paths Paths) (ActiveVersion, error) {
	localVersion, localPath, found, err := FindLocalVersion(cwd)
	if err != nil {
		return ActiveVersion{}, err
	}
	if found {
		return ActiveVersion{Version: localVersion, Scope: ScopeLocal, Source: localPath}, nil
	}

	cfg, err := ReadConfig(paths)
	if err != nil {
		return ActiveVersion{}, err
	}

	if cfg.GlobalVersion == "" {
		return ActiveVersion{}, ErrNoActiveVersion
	}

	normalized, err := versionutil.NormalizeGoVersion(cfg.GlobalVersion)
	if err != nil {
		return ActiveVersion{}, fmt.Errorf("invalid global version in config: %w", err)
	}

	return ActiveVersion{Version: normalized, Scope: ScopeGlobal, Source: paths.ConfigFile}, nil
}

func SetActiveVersion(version string, scope Scope, cwd string, paths Paths) error {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return err
	}

	switch scope {
	case ScopeLocal:
		filePath := filepath.Join(cwd, LocalVersionFile)
		if err := writeFileAtomically(filePath, []byte(normalized+"\n"), 0o644); err != nil {
			return fmt.Errorf("write local version file %s: %w", filePath, err)
		}
		return nil
	case ScopeGlobal:
		cfg, err := ReadConfig(paths)
		if err != nil {
			return err
		}
		cfg.GlobalVersion = normalized
		return WriteConfig(paths, cfg)
	default:
		return fmt.Errorf("unsupported scope %q", scope)
	}
}

func GlobalVersion(paths Paths) (string, bool, error) {
	cfg, err := ReadConfig(paths)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(cfg.GlobalVersion) == "" {
		return "", false, nil
	}
	normalized, err := versionutil.NormalizeGoVersion(cfg.GlobalVersion)
	if err != nil {
		return "", false, fmt.Errorf("invalid global version %q: %w", cfg.GlobalVersion, err)
	}
	return normalized, true, nil
}

func ListInstalledVersions(paths Paths) ([]string, error) {
	if err := EnsureLayout(paths); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(paths.ToolchainsDir)
	if err != nil {
		return nil, fmt.Errorf("read toolchains dir %s: %w", paths.ToolchainsDir, err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		normalized, err := versionutil.NormalizeGoVersion(entry.Name())
		if err != nil {
			continue
		}

		goBinary := filepath.Join(paths.ToolchainsDir, entry.Name(), "bin", "go")
		if _, err := os.Stat(goBinary); err != nil {
			continue
		}

		versions = append(versions, normalized)
	}

	sort.Slice(versions, func(i int, j int) bool {
		cmp, err := versionutil.CompareGoVersions(versions[i], versions[j])
		if err != nil {
			return versions[i] > versions[j]
		}
		return cmp > 0
	})

	return versions, nil
}
