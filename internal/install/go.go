package install

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrtuuro/go-switcher/internal/releases"
	"github.com/mrtuuro/go-switcher/internal/switcher"
	"github.com/mrtuuro/go-switcher/internal/versionutil"
)

const goDownloadBaseURL = "https://go.dev/dl"

func InstallGoArchive(ctx context.Context, paths switcher.Paths, version string, archive releases.File) error {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return err
	}

	if err := switcher.EnsureLayout(paths); err != nil {
		return err
	}

	targetDir := switcher.ToolchainDir(paths, normalized)
	if _, err := os.Stat(filepath.Join(targetDir, "bin", "go")); err == nil {
		return nil
	}

	cachePath := filepath.Join(paths.CacheDir, archive.Filename)
	if err := ensureArchiveInCache(ctx, archive, cachePath); err != nil {
		return err
	}

	if strings.TrimSpace(archive.SHA256) != "" {
		ok, err := verifySHA256(cachePath, archive.SHA256)
		if err != nil {
			return fmt.Errorf("verify checksum for %s: %w", archive.Filename, err)
		}
		if !ok {
			return fmt.Errorf("checksum mismatch for %s", archive.Filename)
		}
	}

	if err := extractGoArchive(cachePath, targetDir); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(targetDir, "bin", "go")); err != nil {
		return fmt.Errorf("installed toolchain %s is missing bin/go", normalized)
	}

	return nil
}

func ensureArchiveInCache(ctx context.Context, archive releases.File, cachePath string) error {
	if _, err := os.Stat(cachePath); err == nil {
		if strings.TrimSpace(archive.SHA256) == "" {
			return nil
		}
		ok, verifyErr := verifySHA256(cachePath, archive.SHA256)
		if verifyErr == nil && ok {
			return nil
		}
		if removeErr := os.Remove(cachePath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("remove bad cached archive %s: %w", cachePath, removeErr)
		}
	}

	url := fmt.Sprintf("%s/%s", goDownloadBaseURL, archive.Filename)
	if err := downloadToFile(ctx, url, cachePath); err != nil {
		return fmt.Errorf("download %s: %w", archive.Filename, err)
	}

	return nil
}

func downloadToFile(ctx context.Context, url string, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create destination parent: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
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
		return fmt.Errorf("perform request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		cleanup()
		return fmt.Errorf("write response body: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(tmpPath, destination); err != nil {
		cleanup()
		return fmt.Errorf("finalize download: %w", err)
	}

	return nil
}

func verifySHA256(filePath string, expectedHex string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false, fmt.Errorf("hash file: %w", err)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	expected := strings.ToLower(strings.TrimSpace(expectedHex))
	return actual == expected, nil
}

func extractGoArchive(archivePath string, targetDir string) error {
	if err := os.RemoveAll(targetDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pre-existing target dir %s: %w", targetDir, err)
	}

	tmpParent := filepath.Dir(targetDir)
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		return fmt.Errorf("create target parent %s: %w", tmpParent, err)
	}

	tmpDir, err := os.MkdirTemp(tmpParent, ".tmp-toolchain-")
	if err != nil {
		return fmt.Errorf("create temp extraction dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %s: %w", archivePath, err)
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

		relativePath, err := stripGoRootPrefix(header.Name)
		if err != nil {
			return err
		}
		if relativePath == "" {
			continue
		}

		targetPath := filepath.Join(tmpDir, relativePath)
		if err := ensureSafePath(tmpDir, targetPath); err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %s: %w", targetPath, err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("close file %s: %w", targetPath, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create symlink parent for %s: %w", targetPath, err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("create symlink %s -> %s: %w", targetPath, header.Linkname, err)
			}
		default:
			continue
		}
	}

	if err := os.Rename(tmpDir, targetDir); err != nil {
		return fmt.Errorf("finalize extraction to %s: %w", targetDir, err)
	}

	return nil
}

func stripGoRootPrefix(path string) (string, error) {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))
	if len(parts) == 0 {
		return "", nil
	}
	if parts[0] != "go" {
		return "", fmt.Errorf("unexpected archive root for %s", path)
	}
	if len(parts) == 1 {
		return "", nil
	}
	return filepath.Join(parts[1:]...), nil
}

func ensureSafePath(baseDir string, targetPath string) error {
	base := filepath.Clean(baseDir)
	target := filepath.Clean(targetPath)
	if target == base {
		return nil
	}
	prefix := base + string(filepath.Separator)
	if !strings.HasPrefix(target, prefix) {
		return fmt.Errorf("unsafe archive path %s", targetPath)
	}
	return nil
}
