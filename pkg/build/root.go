package build

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type TargetType int

const (
	REMOTE TargetType = iota
	LOCAL_DAEMON
	LOCAL_FILE
)

type Platform struct {
	OS      string
	Arch    string
	Variant string
}

func (p Platform) String() string {
	s := p.OS + "/" + p.Arch
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

func (p Platform) ToV1Platform() *v1.Platform {
	return &v1.Platform{
		OS:           p.OS,
		Architecture: p.Arch,
		Variant:      p.Variant,
	}
}

// PlatformSpec holds a platform with optional per-platform overrides.
// Zero-value override fields mean "use the top-level default".
type PlatformSpec struct {
	Platform   Platform
	BaseRef    string
	SourcePath string
	Entrypoint string
	Env        map[string]string
	RunAs      *string
}

type BuildSpecInjectLayer struct {
	Platform Platform

	SourcePath       string
	DestinationPath  string
	DestinationChown bool
	Entrypoint       string
}

type BuildSpecTarget struct {
	Repo string
	Type TargetType
}

type BuildSpec struct {
	BaseRef     string
	InjectLayer BuildSpecInjectLayer
	Target      BuildSpecTarget

	Author      string
	Annotations map[string]string
	Env         map[string]string
	RunAs       *string
}

// MultiPlatformBuildSpec describes a multi-platform build.
// Top-level fields are defaults; PlatformSpec fields override when non-zero.
type MultiPlatformBuildSpec struct {
	BaseRef    string
	Platforms  []PlatformSpec
	SourceRoot string

	DestinationPath  string
	DestinationChown bool
	Entrypoint       string

	Target      BuildSpecTarget
	Author      string
	Annotations map[string]string
	Env         map[string]string
	RunAs       *string
}

type BuildContext struct {
	Context            context.Context
	ExitCleanupWatcher *ExitCleanupWatcher
	Keychain           authn.Keychain

	TempPath string
}

func buildImage(ctx BuildContext, spec BuildSpec) (v1.Image, error) {
	baseImage, baseMetadata, err := getBaseImage(ctx, spec.BaseRef, spec.InjectLayer.Platform, ctx.Keychain)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve base image: %w", err)
	}

	mediaType, err := getMediaType(baseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to get media type: %w", err)
	}

	newLayer, err := createLayerFromFolder(ctx, spec.InjectLayer, tarball.WithMediaType(mediaType))
	if err != nil {
		return nil, fmt.Errorf("failed to create layer from source: %w", err)
	}

	newImage, err := mutate.Append(baseImage, mutate.Addendum{
		Layer:     newLayer,
		MediaType: mediaType,
		History: v1.History{
			Created:   v1.Time{Time: unixEpoch},
			CreatedBy: "tko build",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to append layer to base image: %w", err)
	}

	newImage, err = mutateConfig(newImage, spec, baseMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to mutate config: %w", err)
	}

	return newImage, nil
}

func Build(ctx BuildContext, spec BuildSpec) error {
	image, err := buildImage(ctx, spec)
	if err != nil {
		return err
	}
	return publish(ctx, image, spec.Target)
}

// BuildMultiPlatform builds images for multiple platforms and publishes a manifest list.
func BuildMultiPlatform(ctx BuildContext, spec MultiPlatformBuildSpec) error {
	if spec.Target.Type != REMOTE {
		return fmt.Errorf("multi-platform builds only support REMOTE targets")
	}

	if err := validatePlatformSources(spec); err != nil {
		return err
	}

	// Single platform: build and publish a regular image (not an index)
	if len(spec.Platforms) == 1 {
		resolved := resolvePlatformSpec(spec, spec.Platforms[0])
		return Build(ctx, resolved)
	}

	var addenda []mutate.IndexAddendum
	for _, ps := range spec.Platforms {
		resolved := resolvePlatformSpec(spec, ps)
		log.Printf("Building for platform %s...", ps.Platform)

		img, err := buildImage(ctx, resolved)
		if err != nil {
			return fmt.Errorf("failed to build image for platform %s: %w", ps.Platform, err)
		}

		addenda = append(addenda, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: ps.Platform.ToV1Platform(),
			},
		})
	}

	idx := mutate.AppendManifests(empty.Index, addenda...)

	return publishIndex(ctx, idx, spec.Target)
}

func PlatformSourcePath(sourceRoot string, p Platform) string {
	if p.Variant != "" {
		return filepath.Join(sourceRoot, p.OS, p.Arch, p.Variant)
	}
	return filepath.Join(sourceRoot, p.OS, p.Arch)
}

func resolvePlatformSpec(top MultiPlatformBuildSpec, ps PlatformSpec) BuildSpec {
	baseRef := top.BaseRef
	if ps.BaseRef != "" {
		baseRef = ps.BaseRef
	}

	entrypoint := top.Entrypoint
	if ps.Entrypoint != "" {
		entrypoint = ps.Entrypoint
	}

	sourcePath := PlatformSourcePath(top.SourceRoot, ps.Platform)
	if ps.SourcePath != "" {
		sourcePath = ps.SourcePath
	}

	runAs := top.RunAs
	if ps.RunAs != nil {
		runAs = ps.RunAs
	}

	env := make(map[string]string)
	maps.Copy(env, top.Env)
	maps.Copy(env, ps.Env)

	return BuildSpec{
		BaseRef: baseRef,
		InjectLayer: BuildSpecInjectLayer{
			Platform:         ps.Platform,
			SourcePath:       sourcePath,
			DestinationPath:  top.DestinationPath,
			DestinationChown: top.DestinationChown,
			Entrypoint:       entrypoint,
		},
		Target:      top.Target,
		Author:      top.Author,
		Annotations: top.Annotations,
		Env:         env,
		RunAs:       runAs,
	}
}

func validatePlatformSources(spec MultiPlatformBuildSpec) error {
	for _, ps := range spec.Platforms {
		srcPath := PlatformSourcePath(spec.SourceRoot, ps.Platform)
		if ps.SourcePath != "" {
			srcPath = ps.SourcePath
		}

		info, err := os.Stat(srcPath)
		if err != nil {
			return fmt.Errorf("source directory for platform %s not found: %s\n  Expected directory structure: %s/<os>/<arch>/", ps.Platform, srcPath, spec.SourceRoot)
		}
		if !info.IsDir() {
			return fmt.Errorf("source path for platform %s is not a directory: %s", ps.Platform, srcPath)
		}
	}
	return nil
}

func mutateConfig(img v1.Image, spec BuildSpec, metadata BaseImageMetadata) (v1.Image, error) {
	initImgCfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	imgCfg := initImgCfg.DeepCopy()

	imgCfg.Config.WorkingDir = spec.InjectLayer.DestinationPath
	imgCfg.Config.Entrypoint = []string{spec.InjectLayer.Entrypoint}
	imgCfg.Config.Cmd = nil

	imgCfg.Created = v1.Time{Time: unixEpoch}
	imgCfg.Author = spec.Author
	imgCfg.Container = ""
	imgCfg.DockerVersion = ""

	if spec.RunAs != nil {
		imgCfg.Config.User = *spec.RunAs
	}

	imgCfg.Config.Env = []string{}
	imgCfg.Config.Env = append(imgCfg.Config.Env, initImgCfg.Config.Env...)
	for k, v := range spec.Env {
		imgCfg.Config.Env = append(imgCfg.Config.Env, k+"="+v)
	}

	imgCfg.Config.Labels = map[string]string{}
	imgCfg.Config.Labels["org.opencontainers.image.base.name"] = metadata.name

	if metadata.imageDigest != "" {
		imgCfg.Config.Labels["org.opencontainers.image.base.digest"] = metadata.imageDigest
	}

	maps.Copy(imgCfg.Config.Labels, spec.Annotations)

	return mutate.ConfigFile(img, imgCfg)
}

func ParsePlatform(str string) (Platform, error) {
	parts := strings.Split(str, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return Platform{}, fmt.Errorf("invalid platform string: %s (expected os/arch or os/arch/variant)", str)
	}
	p := Platform{OS: parts[0], Arch: parts[1]}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}

// ParsePlatformSpecs parses a comma-separated list of platform strings into PlatformSpecs.
// Each spec has only the Platform field set (no overrides). Duplicates are rejected.
// Results are sorted by Platform.String() for deterministic output.
func ParsePlatformSpecs(str string) ([]PlatformSpec, error) {
	seen := make(map[string]bool)
	var specs []PlatformSpec

	for _, raw := range strings.Split(str, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p, err := ParsePlatform(raw)
		if err != nil {
			return nil, err
		}
		key := p.String()
		if seen[key] {
			return nil, fmt.Errorf("duplicate platform: %s", key)
		}
		seen[key] = true
		specs = append(specs, PlatformSpec{Platform: p})
	}

	if len(specs) == 0 {
		return nil, fmt.Errorf("no platforms specified")
	}

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Platform.String() < specs[j].Platform.String()
	})

	return specs, nil
}

func ParseTargetType(str string) (TargetType, error) {
	switch str {
	case "REMOTE":
		return REMOTE, nil
	case "LOCAL_DAEMON":
		return LOCAL_DAEMON, nil
	case "LOCAL_FILE":
		return LOCAL_FILE, nil
	case "":
		return REMOTE, nil
	default:
		return -1, fmt.Errorf("invalid target type: %s", str)
	}
}

func getMediaType(base v1.Image) (types.MediaType, error) {
	mt, err := base.MediaType()
	if err != nil {
		return "", err
	}
	switch mt {
	case types.OCIManifestSchema1:
		return types.OCILayer, nil
	case types.DockerManifestSchema2:
		return types.DockerLayer, nil
	}
	return "", fmt.Errorf("unsupported base media type: %s", mt)
}
