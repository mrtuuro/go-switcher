package versionutil

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizeGoVersion normalizes versions like 1.24.2 or go1.24 to go1.24.2.
func NormalizeGoVersion(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("version cannot be empty")
	}

	trimmed = strings.TrimPrefix(trimmed, "go")
	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", fmt.Errorf("invalid go version %q", input)
	}

	numbers := make([]int, 3)
	for i := 0; i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return "", fmt.Errorf("invalid go version %q", input)
		}
		numbers[i] = n
	}

	if len(parts) == 2 {
		numbers[2] = 0
	}

	return fmt.Sprintf("go%d.%d.%d", numbers[0], numbers[1], numbers[2]), nil
}

// ParseGoVersion parses a normalized or raw go version.
func ParseGoVersion(version string) (major int, minor int, patch int, err error) {
	normalized, err := NormalizeGoVersion(version)
	if err != nil {
		return 0, 0, 0, err
	}

	trimmed := strings.TrimPrefix(normalized, "go")
	parts := strings.Split(trimmed, ".")
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse major from %q: %w", version, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse minor from %q: %w", version, err)
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse patch from %q: %w", version, err)
	}

	return major, minor, patch, nil
}

// CompareGoVersions compares go versions and returns -1/0/1.
func CompareGoVersions(a string, b string) (int, error) {
	aMajor, aMinor, aPatch, err := ParseGoVersion(a)
	if err != nil {
		return 0, err
	}
	bMajor, bMinor, bPatch, err := ParseGoVersion(b)
	if err != nil {
		return 0, err
	}

	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1, nil
		}
		return 1, nil
	}

	if aMinor != bMinor {
		if aMinor < bMinor {
			return -1, nil
		}
		return 1, nil
	}

	if aPatch != bPatch {
		if aPatch < bPatch {
			return -1, nil
		}
		return 1, nil
	}

	return 0, nil
}

// CompareDottedVersions compares dotted versions like 1.59.1 and v1.60.0.
func CompareDottedVersions(a string, b string) (int, error) {
	parse := func(v string) ([]int, error) {
		trimmed := strings.TrimPrefix(strings.TrimSpace(v), "v")
		parts := strings.Split(trimmed, ".")
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty dotted version")
		}

		numbers := make([]int, len(parts))
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil, fmt.Errorf("invalid dotted version %q", v)
			}
			numbers[i] = n
		}

		return numbers, nil
	}

	aParts, err := parse(a)
	if err != nil {
		return 0, err
	}
	bParts, err := parse(b)
	if err != nil {
		return 0, err
	}

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		aVal := 0
		if i < len(aParts) {
			aVal = aParts[i]
		}
		bVal := 0
		if i < len(bParts) {
			bVal = bParts[i]
		}

		if aVal < bVal {
			return -1, nil
		}
		if aVal > bVal {
			return 1, nil
		}
	}

	return 0, nil
}
