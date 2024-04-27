package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type TargetType int

const (
	REMOTE TargetType = iota
	LOCAL_DAEMON
)

type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "/" + p.Arch
}

type BuildSpecInjectLayer struct {
	Platform Platform

	SourcePath      string
	DestinationPath string
	Entrypoint      string
}

type BuildSpecTarget struct {
	Repo string
	Type TargetType
}

type BuildSpec struct {
	BaseRef     string
	InjectLayer BuildSpecInjectLayer
	Target      BuildSpecTarget

	Author string
}

type BuildContext struct {
	Context            context.Context
	ExitCleanupWatcher *ExitCleanupWatcher
	Keychain           authn.Keychain

	TempPath string
}

func Build(ctx BuildContext, spec BuildSpec) error {
	baseImage, baseMetadata, err := getBaseImage(ctx, spec.BaseRef, spec.InjectLayer.Platform, ctx.Keychain)
	if err != nil {
		return fmt.Errorf("failed to retrieve base image: %w", err)
	}

	mediaType, err := getMediaType(baseImage)
	if err != nil {
		return fmt.Errorf("failed to get media type: %w", err)
	}

	newLayer, err := createLayerFromFolder(ctx, spec.InjectLayer, tarball.WithMediaType(mediaType))
	if err != nil {
		return fmt.Errorf("failed to create layer from source: %w", err)
	}

	newImage, err := mutate.Append(baseImage, mutate.Addendum{
		Layer:     newLayer,
		MediaType: mediaType,
		History: v1.History{
			Created:   v1.Time{Time: unixEpoch},
			Author:    "github.com/dskiff/tko",
			CreatedBy: "tko build",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to append layer to base image: %w", err)
	}

	newImage, err = mutateConfig(newImage, spec, baseMetadata)
	if err != nil {
		return fmt.Errorf("failed to mutate config: %w", err)
	}

	return publish(ctx, newImage, spec.Target)
}

func mutateConfig(img v1.Image, spec BuildSpec, metadata BaseImageMetadata) (v1.Image, error) {
	imgCfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	imgCfg = imgCfg.DeepCopy()

	imgCfg.Config.WorkingDir = spec.InjectLayer.DestinationPath
	imgCfg.Config.Entrypoint = []string{spec.InjectLayer.Entrypoint}
	imgCfg.Config.Cmd = nil

	imgCfg.Created = v1.Time{Time: unixEpoch}
	imgCfg.Author = spec.Author

	imgCfg.Config.Labels[specsv1.AnnotationBaseImageName] = metadata.name
	imgCfg.Config.Labels[specsv1.AnnotationBaseImageDigest] = metadata.imageDigest

	return mutate.ConfigFile(img, imgCfg)
}

func ParsePlatform(str string) (Platform, error) {
	parts := strings.Split(str, "/")
	if len(parts) != 2 {
		return Platform{}, fmt.Errorf("invalid platform string: %s", str)
	}
	return Platform{
		OS:   parts[0],
		Arch: parts[1],
	}, nil
}

func ParseTargetType(str string) (TargetType, error) {
	switch str {
	case "REMOTE":
		return REMOTE, nil
	case "LOCAL_DAEMON":
		return LOCAL_DAEMON, nil
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
