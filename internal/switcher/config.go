package switcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	GlobalVersion    string            `json:"global_version,omitempty"`
	GolangCILintByGo map[string]string `json:"golangci_lint_by_go,omitempty"`
}

func ReadConfig(paths Paths) (Config, error) {
	if err := EnsureLayout(paths); err != nil {
		return Config{}, err
	}

	raw, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{GolangCILintByGo: map[string]string{}}, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", paths.ConfigFile, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %s: %w", paths.ConfigFile, err)
	}

	if cfg.GolangCILintByGo == nil {
		cfg.GolangCILintByGo = map[string]string{}
	}

	return cfg, nil
}

func WriteConfig(paths Paths, cfg Config) error {
	if err := EnsureLayout(paths); err != nil {
		return err
	}

	if cfg.GolangCILintByGo == nil {
		cfg.GolangCILintByGo = map[string]string{}
	}

	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := writeFileAtomically(paths.ConfigFile, encoded, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", paths.ConfigFile, err)
	}

	return nil
}

func writeFileAtomically(path string, content []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
