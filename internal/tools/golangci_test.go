package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtuuro/go-switcher/internal/switcher"
)

func TestEnsureForGoVersionWithOptions_UpgradesStaleMapping(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	recommended := RecommendedGolangCILint("go1.26.0")
	mustWriteLintBinary(t, paths, recommended)

	cfg := switcher.Config{
		GolangCILintByGo: map[string]string{
			"go1.26.0": "v1.61.0",
		},
	}

	got, err := EnsureForGoVersionWithOptions(context.Background(), paths, &cfg, "go1.26.0", EnsureOptions{})
	if err != nil {
		t.Fatalf("EnsureForGoVersionWithOptions: %v", err)
	}

	if got != recommended {
		t.Fatalf("expected %s, got %s", recommended, got)
	}
	if cfg.GolangCILintByGo["go1.26.0"] != recommended {
		t.Fatalf("expected mapping update to %s, got %s", recommended, cfg.GolangCILintByGo["go1.26.0"])
	}
}

func TestEnsureForGoVersionWithOptions_PreservesNewerMapping(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	mustWriteLintBinary(t, paths, "v9.9.9")

	cfg := switcher.Config{
		GolangCILintByGo: map[string]string{
			"go1.26.0": "v9.9.9",
		},
	}

	got, err := EnsureForGoVersionWithOptions(context.Background(), paths, &cfg, "go1.26.0", EnsureOptions{})
	if err != nil {
		t.Fatalf("EnsureForGoVersionWithOptions: %v", err)
	}

	if got != "v9.9.9" {
		t.Fatalf("expected v9.9.9, got %s", got)
	}
}

func TestEnsureForGoVersionWithOptions_RecoversInvalidMapping(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	recommended := RecommendedGolangCILint("go1.26.0")
	mustWriteLintBinary(t, paths, recommended)

	cfg := switcher.Config{
		GolangCILintByGo: map[string]string{
			"go1.26.0": "latest",
		},
	}

	got, err := EnsureForGoVersionWithOptions(context.Background(), paths, &cfg, "go1.26.0", EnsureOptions{})
	if err != nil {
		t.Fatalf("EnsureForGoVersionWithOptions: %v", err)
	}

	if got != recommended {
		t.Fatalf("expected %s, got %s", recommended, got)
	}
	if cfg.GolangCILintByGo["go1.26.0"] != recommended {
		t.Fatalf("expected mapping update to %s, got %s", recommended, cfg.GolangCILintByGo["go1.26.0"])
	}
}

func testPaths(t *testing.T) switcher.Paths {
	t.Helper()
	tmp := t.TempDir()

	paths := switcher.Paths{
		BaseDir:       filepath.Join(tmp, ".switcher"),
		ToolchainsDir: filepath.Join(tmp, ".switcher", "toolchains"),
		ToolsDir:      filepath.Join(tmp, ".switcher", "tools"),
		BinDir:        filepath.Join(tmp, ".switcher", "bin"),
		CacheDir:      filepath.Join(tmp, ".switcher", "cache"),
		ConfigFile:    filepath.Join(tmp, ".switcher", "config.json"),
	}

	if err := switcher.EnsureLayout(paths); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	return paths
}

func mustWriteLintBinary(t *testing.T, paths switcher.Paths, version string) {
	t.Helper()
	binary := GolangCILintBinaryPath(paths, version)
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(binary, []byte(""), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
