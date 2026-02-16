package versionutil

import "testing"

func TestNormalizeGoVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "already normalized", input: "go1.24.2", want: "go1.24.2"},
		{name: "missing go prefix", input: "1.24.2", want: "go1.24.2"},
		{name: "missing patch", input: "1.25", want: "go1.25.0"},
		{name: "invalid prerelease", input: "go1.25rc1", wantErr: true},
		{name: "invalid text", input: "latest", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeGoVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("NormalizeGoVersion(%q): %v", tc.input, err)
			}

			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestCompareGoVersions(t *testing.T) {
	t.Parallel()

	cmp, err := CompareGoVersions("go1.24.0", "go1.23.9")
	if err != nil {
		t.Fatalf("CompareGoVersions: %v", err)
	}
	if cmp <= 0 {
		t.Fatalf("expected go1.24.0 > go1.23.9")
	}
}

func TestCompareDottedVersions(t *testing.T) {
	t.Parallel()

	cmp, err := CompareDottedVersions("v1.60.3", "1.57.2")
	if err != nil {
		t.Fatalf("CompareDottedVersions: %v", err)
	}
	if cmp <= 0 {
		t.Fatalf("expected v1.60.3 > 1.57.2")
	}
}
