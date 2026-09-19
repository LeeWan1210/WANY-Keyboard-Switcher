package updatecore

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const (
	Owner = "LeeWan1210"
	Repository = "WANY-Keyboard-Switcher"
	Executable = "WANY-Keyboard-Switcher.exe"
	SourceArchive = "WANY-Keyboard-Switcher-source.zip"
	LatestReleaseAPI = "https://api.github.com/repos/" + Owner + "/" + Repository + "/releases/latest"
)

type Asset struct {
	Name string `json:"name"`
	Size int64 `json:"size"`
	Digest string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}
type Release struct {
	TagName string `json:"tag_name"`
	Draft bool `json:"draft"`
	Prerelease bool `json:"prerelease"`
	Assets []Asset `json:"assets"`
}

func parseVersion(tag string) ([3]uint64, error) {
	var parts [3]uint64
	tag = strings.TrimPrefix(strings.TrimSpace(tag), "v")
	pieces := strings.Split(tag, ".")
	if len(pieces) < 2 || len(pieces) > 3 { return parts, errors.New("invalid release version") }
	for i, piece := range pieces {
		if piece == "" { return parts, errors.New("empty release version component") }
		for _, ch := range piece { if ch < '0' || ch > '9' { return parts, errors.New("non-numeric release version") } }
		n, err := strconv.ParseUint(piece, 10, 64)
		if err != nil { return parts, err }
		parts[i] = n
	}
	return parts, nil
}
func IsNewer(current, latest string) (bool, error) {
	a, err := parseVersion(current); if err != nil { return false, err }
	b, err := parseVersion(latest); if err != nil { return false, err }
	for i := range a {
		if b[i] > a[i] { return true, nil }
		if b[i] < a[i] { return false, nil }
	}
	return false, nil
}

// SelectUpdate accepts only the published release's exact, version-free EXE asset.
// Requiring a SHA256 digest prevents installation of incomplete or modified downloads.
func SelectUpdate(r Release) (Asset, error) {
	if r.Draft || r.Prerelease { return Asset{}, errors.New("not a stable published release") }
	if _, err := parseVersion(r.TagName); err != nil { return Asset{}, err }
	for _, a := range r.Assets {
		if a.Name != Executable { continue }
		if a.Size < 100*1024 || a.Size > 50*1024*1024 { return Asset{}, errors.New("unexpected executable size") }
		if len(a.Digest) != len("sha256:")+64 || !strings.HasPrefix(a.Digest, "sha256:") {
			return Asset{}, errors.New("release asset has no valid SHA256 digest")
		}
		for _, ch := range a.Digest[len("sha256:"):] {
			if !(ch >= 'a' && ch <= 'f' || ch >= '0' && ch <= '9') {
				return Asset{}, errors.New("invalid SHA256 digest")
			}
		}
		u, err := url.Parse(a.BrowserDownloadURL)
		if err != nil || u.Scheme != "https" || u.Hostname() != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Asset{}, errors.New("unexpected release download URL")
		}
		want := fmt.Sprintf("/%s/%s/releases/download/%s/%s", Owner, Repository, r.TagName, Executable)
		if path.Clean(u.EscapedPath()) != want {
			return Asset{}, errors.New("download URL does not match the official release")
		}
		return a, nil
	}
	return Asset{}, errors.New("no executable asset found in latest release")
}
