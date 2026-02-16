package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mrtuuro/go-switcher/internal/switcher"
)

func TestDeleteInstalledWithProgress_ActiveLocalSwitchesToNewest(t *testing.T) {
	t.Parallel()

	paths, projectDir := testPaths(t)
	mustWriteToolchain(t, paths, "go1.25.0")
	mustWriteToolchain(t, paths, "go1.24.0")

	localVersionPath := filepath.Join(projectDir, switcher.LocalVersionFile)
	if err := os.WriteFile(localVersionPath, []byte("go1.25.0\n"), 0o644); err != nil {
		t.Fatalf("write local version: %v", err)
	}

	cfg := switcher.Config{
		GolangCILintByGo: map[string]string{
			"go1.25.0": "v1.61.0",
			"go1.24.0": "v1.60.3",
		},
	}
	if err := switcher.WriteConfig(paths, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	mustWriteLintBinary(t, paths, "v1.60.3")

	svc := &Service{Paths: paths}
	result, err := svc.DeleteInstalledWithProgress(context.Background(), projectDir, "go1.25.0", nil)
	if err != nil {
		t.Fatalf("delete installed version: %v", err)
	}

	if !result.WasActive {
		t.Fatalf("expected deleted version to be active")
	}
	if !result.SwitchedToNewest {
		t.Fatalf("expected automatic switch to newest")
	}
	if result.ActiveAfter.Version != "go1.24.0" {
		t.Fatalf("expected active go1.24.0 after delete, got %s", result.ActiveAfter.Version)
	}

	content, err := os.ReadFile(localVersionPath)
	if err != nil {
		t.Fatalf("read local version file: %v", err)
	}
	if string(content) != "go1.24.0\n" {
		t.Fatalf("expected local version file to switch to go1.24.0, got %q", string(content))
	}

	if _, err := os.Stat(switcher.ToolchainDir(paths, "go1.25.0")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted toolchain directory to be removed")
	}
}

func TestDeleteInstalledWithProgress_LastActiveClearsLocalPin(t *testing.T) {
	t.Parallel()

	paths, projectDir := testPaths(t)
	mustWriteToolchain(t, paths, "go1.25.0")

	localVersionPath := filepath.Join(projectDir, switcher.LocalVersionFile)
	if err := os.WriteFile(localVersionPath, []byte("go1.25.0\n"), 0o644); err != nil {
		t.Fatalf("write local version: %v", err)
	}

	svc := &Service{Paths: paths}
	result, err := svc.DeleteInstalledWithProgress(context.Background(), projectDir, "go1.25.0", nil)
	if err != nil {
		t.Fatalf("delete installed version: %v", err)
	}

	if !result.WasActive {
		t.Fatalf("expected deleted version to be active")
	}
	if result.ActiveAfter.Version != "" {
		t.Fatalf("expected no active version after deleting last installed toolchain")
	}
	if _, err := os.Stat(localVersionPath); !os.IsNotExist(err) {
		t.Fatalf("expected local version file to be removed")
	}
}

func testPaths(t *testing.T) (switcher.Paths, string) {
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
		t.Fatalf("ensure layout: %v", err)
	}

	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	return paths, projectDir
}

func mustWriteToolchain(t *testing.T, paths switcher.Paths, version string) {
	t.Helper()
	binDir := filepath.Join(switcher.ToolchainDir(paths, version), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create toolchain bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "go"), []byte(""), 0o755); err != nil {
		t.Fatalf("create go binary: %v", err)
	}
}

func mustWriteLintBinary(t *testing.T, paths switcher.Paths, lintVersion string) {
	t.Helper()
	platformDir := runtime.GOOS + "-" + runtime.GOARCH
	binaryDir := filepath.Join(paths.ToolsDir, "golangci-lint", lintVersion, platformDir)
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		t.Fatalf("create lint binary dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binaryDir, "golangci-lint"), []byte(""), 0o755); err != nil {
		t.Fatalf("create lint binary: %v", err)
	}
}
