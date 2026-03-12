package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlatformTwoSegments(t *testing.T) {
	p, err := ParsePlatform("linux/amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.OS != "linux" || p.Arch != "amd64" || p.Variant != "" {
		t.Fatalf("unexpected platform: %+v", p)
	}
}

func TestParsePlatformThreeSegments(t *testing.T) {
	p, err := ParsePlatform("linux/arm/v7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.OS != "linux" || p.Arch != "arm" || p.Variant != "v7" {
		t.Fatalf("unexpected platform: %+v", p)
	}
}

func TestParsePlatformBackwardCompat(t *testing.T) {
	// Existing test value "custom-os/arch-variant" has one slash, parses as 2 segments
	p, err := ParsePlatform("custom-os/arch-variant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.OS != "custom-os" || p.Arch != "arch-variant" || p.Variant != "" {
		t.Fatalf("unexpected platform: %+v", p)
	}
}

func TestParsePlatformInvalid(t *testing.T) {
	cases := []string{"linux", "a/b/c/d", "", "///", "a//b", "/linux", "linux/"}
	for _, c := range cases {
		_, err := ParsePlatform(c)
		if err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestPlatformString(t *testing.T) {
	cases := []struct {
		p    Platform
		want string
	}{
		{Platform{OS: "linux", Arch: "amd64"}, "linux/amd64"},
		{Platform{OS: "linux", Arch: "arm", Variant: "v7"}, "linux/arm/v7"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Fatalf("got %q, want %q", got, c.want)
		}
	}
}

func TestParsePlatformSpecs(t *testing.T) {
	specs, err := ParsePlatformSpecs("linux/arm64,linux/amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	// Should be sorted: linux/amd64 before linux/arm64
	if specs[0].Platform.String() != "linux/amd64" {
		t.Fatalf("expected linux/amd64 first, got %s", specs[0].Platform)
	}
	if specs[1].Platform.String() != "linux/arm64" {
		t.Fatalf("expected linux/arm64 second, got %s", specs[1].Platform)
	}
}

func TestParsePlatformSpecsDuplicate(t *testing.T) {
	_, err := ParsePlatformSpecs("linux/amd64,linux/amd64")
	if err == nil {
		t.Fatal("expected error for duplicate platforms")
	}
}

func TestParsePlatformSpecsEmpty(t *testing.T) {
	_, err := ParsePlatformSpecs("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestPlatformSourcePath(t *testing.T) {
	cases := []struct {
		root string
		p    Platform
		want string
	}{
		{"./dist", Platform{OS: "linux", Arch: "amd64"}, "dist/linux/amd64"},
		{"./dist", Platform{OS: "linux", Arch: "arm", Variant: "v7"}, "dist/linux/arm/v7"},
		{"/abs/dist", Platform{OS: "linux", Arch: "arm64"}, "/abs/dist/linux/arm64"},
	}
	for _, c := range cases {
		got := PlatformSourcePath(c.root, c.p)
		if got != c.want {
			t.Fatalf("PlatformSourcePath(%q, %s) = %q, want %q", c.root, c.p, got, c.want)
		}
	}
}

func TestResolvePlatformSpec(t *testing.T) {
	topRunAs := "root"
	psRunAs := "nobody"

	top := MultiPlatformBuildSpec{
		BaseRef:          "ubuntu:jammy",
		SourceRoot:       "/src",
		DestinationPath:  "/app",
		DestinationChown: true,
		Entrypoint:       "/app/main",
		Env:              map[string]string{"A": "1", "B": "2"},
		RunAs:            &topRunAs,
		Author:           "test",
		Target:           BuildSpecTarget{Repo: "repo", Type: REMOTE},
	}

	ps := PlatformSpec{
		Platform:   Platform{OS: "linux", Arch: "arm64"},
		BaseRef:    "alpine:latest",
		Entrypoint: "/app/alt",
		Env:        map[string]string{"B": "override", "C": "3"},
		RunAs:      &psRunAs,
	}

	resolved := resolvePlatformSpec(top, ps)

	if resolved.BaseRef != "alpine:latest" {
		t.Fatalf("expected BaseRef override, got %q", resolved.BaseRef)
	}
	if resolved.InjectLayer.Entrypoint != "/app/alt" {
		t.Fatalf("expected Entrypoint override, got %q", resolved.InjectLayer.Entrypoint)
	}
	if resolved.InjectLayer.SourcePath != "/src/linux/arm64" {
		t.Fatalf("expected convention source path, got %q", resolved.InjectLayer.SourcePath)
	}
	if *resolved.RunAs != "nobody" {
		t.Fatalf("expected RunAs override, got %q", *resolved.RunAs)
	}
	if resolved.Env["A"] != "1" {
		t.Fatalf("expected inherited env A=1, got %q", resolved.Env["A"])
	}
	if resolved.Env["B"] != "override" {
		t.Fatalf("expected overridden env B=override, got %q", resolved.Env["B"])
	}
	if resolved.Env["C"] != "3" {
		t.Fatalf("expected platform env C=3, got %q", resolved.Env["C"])
	}
}

func TestResolvePlatformSpecSourcePathOverride(t *testing.T) {
	top := MultiPlatformBuildSpec{
		SourceRoot: "/src",
	}
	ps := PlatformSpec{
		Platform:   Platform{OS: "linux", Arch: "amd64"},
		SourcePath: "/custom/path",
	}
	resolved := resolvePlatformSpec(top, ps)
	if resolved.InjectLayer.SourcePath != "/custom/path" {
		t.Fatalf("expected SourcePath override, got %q", resolved.InjectLayer.SourcePath)
	}
}

func TestValidatePlatformSources(t *testing.T) {
	dir := t.TempDir()
	// Create linux/amd64 but not linux/arm64
	if err := os.MkdirAll(filepath.Join(dir, "linux", "amd64"), 0o755); err != nil {
		t.Fatal(err)
	}

	spec := MultiPlatformBuildSpec{
		SourceRoot: dir,
		Platforms: []PlatformSpec{
			{Platform: Platform{OS: "linux", Arch: "amd64"}},
			{Platform: Platform{OS: "linux", Arch: "arm64"}},
		},
	}

	err := validatePlatformSources(spec)
	if err == nil {
		t.Fatal("expected error for missing arm64 directory")
	}
}

func TestValidatePlatformSourcesAllPresent(t *testing.T) {
	dir := t.TempDir()
	for _, arch := range []string{"amd64", "arm64"} {
		if err := os.MkdirAll(filepath.Join(dir, "linux", arch), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	spec := MultiPlatformBuildSpec{
		SourceRoot: dir,
		Platforms: []PlatformSpec{
			{Platform: Platform{OS: "linux", Arch: "amd64"}},
			{Platform: Platform{OS: "linux", Arch: "arm64"}},
		},
	}

	if err := validatePlatformSources(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
