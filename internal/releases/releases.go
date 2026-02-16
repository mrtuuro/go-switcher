package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mrtuuro/go-switcher/internal/versionutil"
)

const DefaultURL = "https://go.dev/dl/?mode=json&include=all"

type Client struct {
	URL        string
	HTTPClient *http.Client
}

type Release struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Files   []File `json:"files"`
}

type File struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
}

func NewClient() *Client {
	return &Client{
		URL: DefaultURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Fetch(ctx context.Context) ([]Release, error) {
	url := c.URL
	if strings.TrimSpace(url) == "" {
		url = DefaultURL
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create releases request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch releases returned status %d", resp.StatusCode)
	}

	var all []Release
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("decode releases response: %w", err)
	}

	return all, nil
}

func (r Release) ArchiveFor(goos string, goarch string) (File, bool) {
	for _, f := range r.Files {
		if f.Kind != "archive" {
			continue
		}
		if f.OS != goos || f.Arch != goarch {
			continue
		}
		if !strings.HasSuffix(f.Filename, ".tar.gz") {
			continue
		}
		return f, true
	}

	return File{}, false
}

func AvailableVersions(all []Release, goos string, goarch string) []string {
	if strings.TrimSpace(goos) == "" {
		goos = runtime.GOOS
	}
	if strings.TrimSpace(goarch) == "" {
		goarch = runtime.GOARCH
	}

	set := map[string]struct{}{}
	for _, r := range all {
		if _, ok := r.ArchiveFor(goos, goarch); !ok {
			continue
		}

		normalized, err := versionutil.NormalizeGoVersion(r.Version)
		if err != nil {
			continue
		}
		set[normalized] = struct{}{}
	}

	versions := make([]string, 0, len(set))
	for v := range set {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i int, j int) bool {
		cmp, err := versionutil.CompareGoVersions(versions[i], versions[j])
		if err != nil {
			return versions[i] > versions[j]
		}
		return cmp > 0
	})

	return versions
}

func FindArchive(all []Release, version string, goos string, goarch string) (File, string, error) {
	normalized, err := versionutil.NormalizeGoVersion(version)
	if err != nil {
		return File{}, "", err
	}

	if strings.TrimSpace(goos) == "" {
		goos = runtime.GOOS
	}
	if strings.TrimSpace(goarch) == "" {
		goarch = runtime.GOARCH
	}

	for _, r := range all {
		releaseVersion, err := versionutil.NormalizeGoVersion(r.Version)
		if err != nil {
			continue
		}
		if releaseVersion != normalized {
			continue
		}
		archive, ok := r.ArchiveFor(goos, goarch)
		if !ok {
			return File{}, "", fmt.Errorf("%s is not available for %s/%s", normalized, goos, goarch)
		}
		return archive, normalized, nil
	}

	return File{}, "", fmt.Errorf("go release %s not found", normalized)
}
