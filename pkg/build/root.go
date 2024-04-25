package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
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
}

type BuildContext struct {
	Context            context.Context
	ExitCleanupWatcher *ExitCleanupWatcher
	Keychain           authn.Keychain

	TempPath string
}

func Build(ctx BuildContext, spec BuildSpec) error {
	baseImage, err := getBaseImage(ctx, spec.BaseRef, spec.InjectLayer.Platform, ctx.Keychain)
	if err != nil {
		return fmt.Errorf("failed to retrieve base image: %w", err)
	}

	newLayer, err := createLayerFromFolder(ctx, spec.InjectLayer)
	if err != nil {
		return fmt.Errorf("failed to create layer from source: %w", err)
	}

	newImage, err := mutate.AppendLayers(baseImage, newLayer)
	if err != nil {
		return fmt.Errorf("failed to append layer to base image: %w", err)
	}

	newImage, err = mutateConfig(newImage, spec.InjectLayer)
	if err != nil {
		return fmt.Errorf("failed to mutate config: %w", err)
	}

	return publish(ctx, newImage, spec.Target)
}

func mutateConfig(img v1.Image, layer BuildSpecInjectLayer) (v1.Image, error) {
	imgCfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	imgCfg = imgCfg.DeepCopy()

	imgCfg.Config.WorkingDir = layer.DestinationPath
	imgCfg.Config.Entrypoint = []string{layer.Entrypoint}
	imgCfg.Config.Cmd = nil

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
