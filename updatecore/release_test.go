package updatecore

import (
	"strings"
	"testing"
)

func TestVersionComparison(t *testing.T) {
	for _, tc := range []struct{ current, latest string; want bool }{
		{"v0.5", "v0.6", true},
		{"v0.5", "v0.5.1", true},
		{"v0.5.0", "v0.5", false},
		{"v0.5", "v0.4", false},
		{"v1.2.10", "v1.2.9", false},
		{"v1.2.9", "v1.2.10", true},
		{"v9.9", "v10.0", true},
	} {
		got, err := IsNewer(tc.current, tc.latest)
		if err != nil || got != tc.want {
			t.Fatalf("%s -> %s: newer=%v error=%v, want=%v", tc.current, tc.latest, got, err, tc.want)
		}
	}
	for _, tag := range []string{"", "beta", "v0.5-beta", "v0.5.0.1", "v", "v0.-1", "v0.999999999999999999999999999999"} {
		if _, err := IsNewer("v0.5", tag); err == nil { t.Errorf("accepted invalid tag %q", tag) }
	}
}
func validRelease() Release {
	tag := "v0.6"
	url := "https://github.com/" + Owner + "/" + Repository + "/releases/download/" + tag + "/" + Executable
	return Release{TagName: tag, Assets: []Asset{{Name: Executable, Size: 2246144, Digest: "sha256:" + strings.Repeat("a", 64), BrowserDownloadURL: url}}}
}
func TestSelectUpdate(t *testing.T) {
	r := validRelease()
	if a, err := SelectUpdate(r); err != nil || a.Name != Executable { t.Fatalf("valid asset: %v",err) }
	r.Prerelease = true
	if _, err := SelectUpdate(r); err == nil { t.Fatal("accepted prerelease") }
	r = validRelease(); r.Assets[0].Digest = ""
	if _, err := SelectUpdate(r); err == nil { t.Fatal("accepted unsigned asset digest") }
	r = validRelease(); r.Assets[0].BrowserDownloadURL = "https://attacker.example/file.exe"
	if _, err := SelectUpdate(r); err == nil { t.Fatal("accepted foreign host") }
	r = validRelease(); r.Assets[0].BrowserDownloadURL = "https://github.com/"+Owner+"/"+Repository+"/releases/download/v0.5/"+Executable
	if _, err := SelectUpdate(r); err == nil { t.Fatal("accepted mismatched tag") }
	r = validRelease(); r.Assets[0].Name = "WANY-Keyboard-Switcher-v0.6.exe"
	if _, err := SelectUpdate(r); err == nil { t.Fatal("accepted versioned or different EXE asset") }
	r = validRelease(); r.Assets[0].Size = 100*1024*1024
	if _, err := SelectUpdate(r); err == nil { t.Fatal("accepted oversized EXE") }
}
