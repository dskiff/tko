package build

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
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
	Ctx                context.Context
	ExitCleanupWatcher *ExitCleanupWatcher
	Keychain           authn.Keychain

	TempPath string
}

func Build(ctx BuildContext, cfg BuildSpec) error {
	tag, err := name.NewTag(cfg.Target.Repo)
	if err != nil {
		return fmt.Errorf("failed to parse target repo: %w", err)
	}

	baseImage, err := GetBaseImage(ctx, cfg.BaseRef, cfg.InjectLayer.Platform, ctx.Keychain)
	if err != nil {
		return fmt.Errorf("failed to retrieve base image: %w", err)
	}

	newLayer, err := createLayerFromFolder(ctx, cfg.InjectLayer.SourcePath, cfg.InjectLayer.DestinationPath)
	if err != nil {
		return fmt.Errorf("failed to create layer from source: %w", err)
	}

	newImage, err := mutate.AppendLayers(baseImage, newLayer)
	if err != nil {
		return fmt.Errorf("failed to append layer to base image: %w", err)
	}

	newImage, err = mutateConfig(newImage, cfg)
	if err != nil {
		return fmt.Errorf("failed to mutate config: %w", err)
	}

	return Publish(ctx, tag, newImage, cfg.Target)
}

func mutateConfig(img v1.Image, runCfg BuildSpec) (v1.Image, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg = cfg.DeepCopy()

	cfg.Config.WorkingDir = runCfg.InjectLayer.DestinationPath
	cfg.Config.Entrypoint = []string{runCfg.InjectLayer.Entrypoint}
	cfg.Config.Cmd = nil

	return mutate.ConfigFile(img, cfg)
}
