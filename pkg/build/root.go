package build

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

	BaseImage  string
	TargetRepo string
	TargetType TargetType

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

	baseRef, baseIndex, err := fetchImageIndex(ctx, cfg.BaseImage)
	if err != nil {
		return fmt.Errorf("failed to retrieve base image index: %w", err)
	}
	baseDigest, err := baseIndex.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve base image digest: %w", err)
	}
	log.Println("Using base image:", baseRef.Name()+"@"+baseDigest.String())

	baseImage, err := getImageForPlatform(baseIndex, cfg.PlatformArch, cfg.PlatformOs)
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

	newImageDigest, err := newImage.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve new image digest: %w", err)
	}
	log.Println("Created new image:", newImageDigest)

	switch cfg.TargetType {
	case REMOTE:
		log.Println("Publishing to remote...")
		err := remote.Write(tag, newImage, remote.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to write image to remote: %w", err)
		}
	case LOCAL_DAEMON:
		log.Println("Publishing to local daemon...")
		_, err := daemon.Write(tag, newImage, daemon.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to write image to daemon: %w", err)
		}
	default:
		return fmt.Errorf("unknown target type: %d", cfg.TargetType)
	}

	return nil
}

func fetchImageIndex(ctx context.Context, src string) (name.Reference, v1.ImageIndex, error) {
	ref, err := name.ParseReference(src)
	if err != nil {
		return nil, nil, err
	}
	base, err := remote.Index(ref, remote.WithContext(ctx))
	return ref, base, err
}

func getDigestForPlatform(index v1.ImageIndex, arch string, os string) (v1.Hash, error) {
	manifest, err := index.IndexManifest()
	if err != nil {
		return v1.Hash{}, err
	}

	// Find the manifest for the platform
	for _, m := range manifest.Manifests {
		if m.Platform.Architecture == arch && m.Platform.OS == os {
			return m.Digest, nil
		}
	}

	return v1.Hash{}, fmt.Errorf("no manifest found for platform %s/%s", arch, os)
}

func getImageForPlatform(index v1.ImageIndex, arch string, os string) (v1.Image, error) {
	digest, err := getDigestForPlatform(index, arch, os)
	if err != nil {
		return nil, err
	}
	return index.Image(digest)
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
