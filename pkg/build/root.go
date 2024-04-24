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

type RunConfig struct {
	SrcPath    string
	DstPath    string
	Entrypoint string

	BaseImage      string
	TargetRepo     string
	TargetType     TargetType
	RemoteKeychain authn.Keychain

	PlatformOs   string
	PlatformArch string

	TempPath           string
	ExitCleanupWatcher *ExitCleanupWatcher
}

func Run(ctx context.Context, cfg RunConfig) error {
	tag, err := name.NewTag(cfg.TargetRepo)
	if err != nil {
		return fmt.Errorf("failed to parse target repo: %w", err)
	}

	baseImage, err := GetBaseImage(ctx, cfg.BaseImage, cfg)
	if err != nil {
		return fmt.Errorf("failed to retrieve base image: %w", err)
	}

	newLayer, err := createLayerFromFolder(cfg.SrcPath, cfg.DstPath, cfg)
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

	return Publish(ctx, tag, newImage, cfg)
}

func mutateConfig(img v1.Image, runCfg RunConfig) (v1.Image, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg = cfg.DeepCopy()

	cfg.Config.Entrypoint = []string{runCfg.Entrypoint}
	cfg.Config.Cmd = nil
	cfg.Config.WorkingDir = runCfg.DstPath

	return mutate.ConfigFile(img, cfg)
}
