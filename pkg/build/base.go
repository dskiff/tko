package build

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func GetBaseImage(ctx context.Context, baseName string, cfg RunConfig) (v1.Image, error) {
	if baseName == "scratch" {
		return empty.Image, nil
	}

	baseRef, baseIndex, err := fetchImageIndex(ctx, baseName, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve base image index: %w", err)
	}
	baseDigest, err := baseIndex.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve base image digest: %w", err)
	}
	log.Println("Using base image:", baseRef.Name()+"@"+baseDigest.String())

	return getImageForPlatform(baseIndex, cfg.PlatformArch, cfg.PlatformOs)
}

func fetchImageIndex(ctx context.Context, src string, cfg RunConfig) (name.Reference, v1.ImageIndex, error) {
	ref, err := name.ParseReference(src)
	if err != nil {
		return nil, nil, err
	}
	base, err := remote.Index(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(cfg.RemoteKeychain))
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
