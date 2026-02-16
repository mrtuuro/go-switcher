package switcher

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	BaseDir       string
	ToolchainsDir string
	ToolsDir      string
	BinDir        string
	CacheDir      string
	ConfigFile    string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home: %w", err)
	}

	base := filepath.Join(home, ".switcher")
	return Paths{
		BaseDir:       base,
		ToolchainsDir: filepath.Join(base, "toolchains"),
		ToolsDir:      filepath.Join(base, "tools"),
		BinDir:        filepath.Join(base, "bin"),
		CacheDir:      filepath.Join(base, "cache"),
		ConfigFile:    filepath.Join(base, "config.json"),
	}, nil
}

func EnsureLayout(paths Paths) error {
	dirs := []string{
		paths.BaseDir,
		paths.ToolchainsDir,
		paths.ToolsDir,
		paths.BinDir,
		paths.CacheDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	return nil
}

func ToolchainDir(paths Paths, goVersion string) string {
	return filepath.Join(paths.ToolchainsDir, goVersion)
}

func ToolchainExists(paths Paths, goVersion string) bool {
	_, err := os.Stat(filepath.Join(ToolchainDir(paths, goVersion), "bin", "go"))
	return err == nil
}

func GoToolBinary(paths Paths, goVersion string, tool string) (string, error) {
	if tool != "go" && tool != "gofmt" {
		return "", fmt.Errorf("unsupported go tool %q", tool)
	}

	binary := filepath.Join(ToolchainDir(paths, goVersion), "bin", tool)
	if _, err := os.Stat(binary); err != nil {
		return "", fmt.Errorf("%s binary for %s not found at %s", tool, goVersion, binary)
	}

	return binary, nil
}
