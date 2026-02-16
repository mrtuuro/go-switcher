package tools

import (
	"strings"

	"github.com/mrtuuro/go-switcher/internal/versionutil"
)

type compatibilityRule struct {
	MinGo       string
	MaxGo       string
	LintVersion string
}

var compatibilityRules = []compatibilityRule{
	{MinGo: "go1.0.0", MaxGo: "go1.20.99", LintVersion: "v1.54.2"},
	{MinGo: "go1.21.0", MaxGo: "go1.22.99", LintVersion: "v1.57.2"},
	{MinGo: "go1.23.0", MaxGo: "go1.24.99", LintVersion: "v1.64.8"},
	{MinGo: "go1.25.0", MaxGo: "", LintVersion: "v2.9.0"},
}

func RecommendedGolangCILint(goVersion string) string {
	normalized, err := versionutil.NormalizeGoVersion(goVersion)
	if err != nil {
		return compatibilityRules[len(compatibilityRules)-1].LintVersion
	}

	for _, rule := range compatibilityRules {
		if !isWithinRange(normalized, rule.MinGo, rule.MaxGo) {
			continue
		}
		return rule.LintVersion
	}

	return compatibilityRules[len(compatibilityRules)-1].LintVersion
}

func isWithinRange(value string, min string, max string) bool {
	if strings.TrimSpace(min) != "" {
		cmp, err := versionutil.CompareGoVersions(value, min)
		if err != nil || cmp < 0 {
			return false
		}
	}

	if strings.TrimSpace(max) != "" {
		cmp, err := versionutil.CompareGoVersions(value, max)
		if err != nil || cmp > 0 {
			return false
		}
	}

	return true
}
