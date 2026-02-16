package tools

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mrtuuro/go-switcher/internal/switcher"
)

func GolangCILintBinaryPath(paths switcher.Paths, lintVersion string) string {
	platformDir := runtime.GOOS + "-" + runtime.GOARCH
	return filepath.Join(paths.ToolsDir, "golangci-lint", lintVersion, platformDir, "golangci-lint")
}

func EnsureForGoVersion(ctx context.Context, paths switcher.Paths, cfg *switcher.Config, goVersion string) (string, error) {
	if cfg.GolangCILintByGo == nil {
		cfg.GolangCILintByGo = map[string]string{}
	}

	lintVersion := cfg.GolangCILintByGo[goVersion]
	if strings.TrimSpace(lintVersion) == "" {
		lintVersion = RecommendedGolangCILint(goVersion)
		cfg.GolangCILintByGo[goVersion] = lintVersion
	}

	binaryPath := GolangCILintBinaryPath(paths, lintVersion)
	if _, err := os.Stat(binaryPath); err == nil {
		return lintVersion, nil
	}

	if err := installGolangCILint(ctx, paths, lintVersion); err != nil {
		return "", err
	}

	return lintVersion, nil
}

func ResolveBinary(paths switcher.Paths, cfg switcher.Config, goVersion string) (binaryPath string, lintVersion string, err error) {
	lintVersion = cfg.GolangCILintByGo[goVersion]
	if strings.TrimSpace(lintVersion) == "" {
		lintVersion = RecommendedGolangCILint(goVersion)
	}

	binaryPath = GolangCILintBinaryPath(paths, lintVersion)
	if _, statErr := os.Stat(binaryPath); statErr != nil {
		return "", lintVersion, fmt.Errorf("golangci-lint %s is not installed for %s (expected %s)", lintVersion, goVersion, binaryPath)
	}

	return binaryPath, lintVersion, nil
}

func installGolangCILint(ctx context.Context, paths switcher.Paths, lintVersion string) error {
	if err := switcher.EnsureLayout(paths); err != nil {
		return err
	}

	versionNoPrefix := strings.TrimPrefix(lintVersion, "v")
	archiveName := fmt.Sprintf("golangci-lint-%s-%s-%s.tar.gz", versionNoPrefix, runtime.GOOS, runtime.GOARCH)
	archiveURL := fmt.Sprintf("https://github.com/golangci/golangci-lint/releases/download/%s/%s", lintVersion, archiveName)
	cachePath := filepath.Join(paths.CacheDir, archiveName)
	if _, err := os.Stat(cachePath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat cache file %s: %w", cachePath, err)
		}
		if err := downloadToFile(ctx, archiveURL, cachePath); err != nil {
			return fmt.Errorf("download golangci-lint archive: %w", err)
		}
	}

	binaryPath := GolangCILintBinaryPath(paths, lintVersion)
	if err := extractBinaryFromArchive(cachePath, binaryPath, "golangci-lint"); err != nil {
		return fmt.Errorf("install golangci-lint %s: %w", lintVersion, err)
	}

	return nil
}

func downloadToFile(ctx context.Context, url string, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cleanup()
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		cleanup()
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		cleanup()
		return fmt.Errorf("write response: %w", err)
	}

	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(tmpPath, destination); err != nil {
		cleanup()
		return fmt.Errorf("finalize download: %w", err)
	}

	return nil
}

func extractBinaryFromArchive(archivePath string, destination string, binaryName string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create binary destination directory: %w", err)
	}

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() {
		_ = archiveFile.Close()
	}()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() {
		_ = gzReader.Close()
	}()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if !strings.HasSuffix(header.Name, "/"+binaryName) && filepath.Base(header.Name) != binaryName {
			continue
		}

		tmpFile, err := os.CreateTemp(filepath.Dir(destination), ".tmp-golangci-*")
		if err != nil {
			return fmt.Errorf("create temp binary file: %w", err)
		}
		tmpPath := tmpFile.Name()

		cleanup := func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}

		if _, err := io.Copy(tmpFile, tarReader); err != nil {
			cleanup()
			return fmt.Errorf("write temporary binary: %w", err)
		}
		if err := tmpFile.Chmod(0o755); err != nil {
			cleanup()
			return fmt.Errorf("set executable bit: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			cleanup()
			return fmt.Errorf("close temporary binary: %w", err)
		}

		if err := os.Rename(tmpPath, destination); err != nil {
			cleanup()
			return fmt.Errorf("finalize binary install: %w", err)
		}

		return nil
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}
