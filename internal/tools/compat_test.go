package tools

import "testing"

func TestRecommendedGolangCILint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goVersion string
		want      string
	}{
		{goVersion: "go1.20.10", want: "v1.54.2"},
		{goVersion: "go1.21.8", want: "v1.57.2"},
		{goVersion: "go1.24.1", want: "v1.64.8"},
		{goVersion: "go1.25.0", want: "v2.9.0"},
		{goVersion: "go1.26.3", want: "v2.9.0"},
	}

	for _, tc := range tests {
		got := RecommendedGolangCILint(tc.goVersion)
		if got != tc.want {
			t.Fatalf("for %s expected %s, got %s", tc.goVersion, tc.want, got)
		}
	}
}
