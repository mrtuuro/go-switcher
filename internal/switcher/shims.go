package switcher

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var shimTools = []string{"go", "gofmt", "golangci-lint"}

func EnsureShims(paths Paths) error {
	if err := EnsureLayout(paths); err != nil {
		return err
	}

	if err := ensureSwitcherBinary(paths); err != nil {
		return err
	}

	for _, tool := range shimTools {
		shimPath := filepath.Join(paths.BinDir, tool)
		script := shimScript(tool)
		if err := writeFileAtomically(shimPath, []byte(script), 0o755); err != nil {
			return fmt.Errorf("write shim %s: %w", shimPath, err)
		}
	}

	return nil
}

func shimScript(tool string) string {
	return fmt.Sprintf(`#!/usr/bin/env sh
set -eu

switcher_bin="$(dirname "$0")/switcher"

if [ ! -x "$switcher_bin" ]; then
  echo "switcher binary not found at $switcher_bin" >&2
  echo "Run 'switcher use <version>' once to bootstrap shims." >&2
  exit 1
fi

exec "$switcher_bin" exec %s "$@"
`, tool)
}

func ensureSwitcherBinary(paths Paths) error {
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	resolvedPath := executablePath
	if evaluatedPath, evalErr := filepath.EvalSymlinks(executablePath); evalErr == nil {
		resolvedPath = evaluatedPath
	}

	targetPath := filepath.Join(paths.BinDir, "switcher")
	if sameFile(targetPath, resolvedPath) {
		return nil
	}

	return copyExecutable(resolvedPath, targetPath)
}

func sameFile(a string, b string) bool {
	aInfo, aErr := os.Stat(a)
	if aErr != nil {
		return false
	}
	bInfo, bErr := os.Stat(b)
	if bErr != nil {
		return false
	}
	return os.SameFile(aInfo, bInfo)
}

func copyExecutable(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source executable %s: %w", sourcePath, err)
	}
	defer func() {
		_ = source.Close()
	}()

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory %s: %w", targetDir, err)
	}

	tmpFile, err := os.CreateTemp(targetDir, ".switcher-*")
	if err != nil {
		return fmt.Errorf("create temporary executable file: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := io.Copy(tmpFile, source); err != nil {
		cleanup()
		return fmt.Errorf("copy executable bytes: %w", err)
	}

	if err := tmpFile.Chmod(0o755); err != nil {
		cleanup()
		return fmt.Errorf("mark executable: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temporary executable: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		cleanup()
		return fmt.Errorf("install executable at %s: %w", targetPath, err)
	}

	return nil
}

func ShimTools() []string {
	copySlice := make([]string, len(shimTools))
	copy(copySlice, shimTools)
	return copySlice
}

func EnsurePathHint(paths Paths) (string, bool, error) {
	currentPath := os.Getenv("PATH")
	if currentPath == "" {
		return paths.BinDir, false, nil
	}

	for _, segment := range filepath.SplitList(currentPath) {
		if segment == paths.BinDir {
			return paths.BinDir, true, nil
		}
	}

	return paths.BinDir, false, nil
}
