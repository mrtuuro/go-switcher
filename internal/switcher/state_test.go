package switcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveActiveVersion_LocalTakesPrecedence(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	paths := Paths{
		BaseDir:       filepath.Join(tmp, ".switcher"),
		ToolchainsDir: filepath.Join(tmp, ".switcher", "toolchains"),
		ToolsDir:      filepath.Join(tmp, ".switcher", "tools"),
		BinDir:        filepath.Join(tmp, ".switcher", "bin"),
		CacheDir:      filepath.Join(tmp, ".switcher", "cache"),
		ConfigFile:    filepath.Join(tmp, ".switcher", "config.json"),
	}

	if err := EnsureLayout(paths); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	if err := WriteConfig(paths, Config{GlobalVersion: "go1.24.0"}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	localPath := filepath.Join(projectDir, LocalVersionFile)
	if err := os.WriteFile(localPath, []byte("go1.23.1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	resolved, err := ResolveActiveVersion(projectDir, paths)
	if err != nil {
		t.Fatalf("ResolveActiveVersion: %v", err)
	}

	if resolved.Version != "go1.23.1" {
		t.Fatalf("expected local version go1.23.1, got %s", resolved.Version)
	}
	if resolved.Scope != ScopeLocal {
		t.Fatalf("expected scope local, got %s", resolved.Scope)
	}
	if resolved.Source != localPath {
		t.Fatalf("expected source %s, got %s", localPath, resolved.Source)
	}
}

func TestSetActiveVersion_LocalWritesFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	paths := Paths{
		BaseDir:       filepath.Join(tmp, ".switcher"),
		ToolchainsDir: filepath.Join(tmp, ".switcher", "toolchains"),
		ToolsDir:      filepath.Join(tmp, ".switcher", "tools"),
		BinDir:        filepath.Join(tmp, ".switcher", "bin"),
		CacheDir:      filepath.Join(tmp, ".switcher", "cache"),
		ConfigFile:    filepath.Join(tmp, ".switcher", "config.json"),
	}

	projectDir := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := SetActiveVersion("1.25", ScopeLocal, projectDir, paths); err != nil {
		t.Fatalf("SetActiveVersion local: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(projectDir, LocalVersionFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(content) != "go1.25.0\n" {
		t.Fatalf("expected local file content go1.25.0, got %q", string(content))
	}
}

func TestListInstalledVersions_SortsDescending(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	paths := Paths{
		BaseDir:       filepath.Join(tmp, ".switcher"),
		ToolchainsDir: filepath.Join(tmp, ".switcher", "toolchains"),
		ToolsDir:      filepath.Join(tmp, ".switcher", "tools"),
		BinDir:        filepath.Join(tmp, ".switcher", "bin"),
		CacheDir:      filepath.Join(tmp, ".switcher", "cache"),
		ConfigFile:    filepath.Join(tmp, ".switcher", "config.json"),
	}

	versions := []string{"go1.23.5", "go1.25.0", "go1.24.2"}
	for _, v := range versions {
		binDir := filepath.Join(paths.ToolchainsDir, v, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(binDir, "go"), []byte(""), 0o755); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	sorted, err := ListInstalledVersions(paths)
	if err != nil {
		t.Fatalf("ListInstalledVersions: %v", err)
	}

	expected := []string{"go1.25.0", "go1.24.2", "go1.23.5"}
	if len(sorted) != len(expected) {
		t.Fatalf("expected %d versions, got %d", len(expected), len(sorted))
	}
	for i := range expected {
		if sorted[i] != expected[i] {
			t.Fatalf("expected %s at index %d, got %s", expected[i], i, sorted[i])
		}
	}
}
