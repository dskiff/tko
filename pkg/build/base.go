package build

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type BaseImageMetadata struct {
	name        string
	imageDigest string
}

func getBaseImage(ctx BuildContext, baseRef string, platform Platform, keychain authn.Keychain) (v1.Image, BaseImageMetadata, error) {
	if baseRef == "scratch" {
		return empty.Image, BaseImageMetadata{
			name: "scratch",
		}, nil
	}

	ref, err := name.ParseReference(baseRef)
	if err != nil {
		return nil, BaseImageMetadata{}, fmt.Errorf("failed to parse base image reference: %w", err)
	}

	desc, err := remote.Get(ref, remote.WithContext(ctx.Context), remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, BaseImageMetadata{}, fmt.Errorf("failed to retrieve base image: %w", err)
	}

	log.Println("Using base image:", ref.Context().Digest(desc.Digest.String()))

	var img v1.Image
	switch {
	case desc.MediaType.IsIndex():
		index, err := desc.ImageIndex()
		if err != nil {
			return nil, BaseImageMetadata{}, fmt.Errorf("failed to retrieve base image index: %w", err)
		}
		img, err = getImageForPlatform(index, platform)
		if err != nil {
			return nil, BaseImageMetadata{}, fmt.Errorf("failed to retrieve base image for platform: %w", err)
		}
	case desc.MediaType.IsImage():
		img, err = desc.Image()
		if err != nil {
			return nil, BaseImageMetadata{}, fmt.Errorf("failed to retrieve base image: %w", err)
		}
		if err := verifyImagePlatform(img, platform); err != nil {
			return nil, BaseImageMetadata{}, err
		}
	default:
		return nil, BaseImageMetadata{}, fmt.Errorf("unsupported base image media type: %s", desc.MediaType)
	}

	imgDigest, err := img.Digest()
	if err != nil {
		return nil, BaseImageMetadata{}, fmt.Errorf("failed to retrieve base image digest: %w", err)
	}

	return img, BaseImageMetadata{
		name:        ref.Context().Name(),
		imageDigest: imgDigest.String(),
	}, nil
}

func getImageForPlatform(index v1.ImageIndex, platform Platform) (v1.Image, error) {
	digest, err := getDigestForPlatform(index, platform)
	if err != nil {
		return nil, err
	}
	return index.Image(digest)
}

func getDigestForPlatform(index v1.ImageIndex, platform Platform) (v1.Hash, error) {
	manifest, err := index.IndexManifest()
	if err != nil {
		return v1.Hash{}, err
	}

	// Find the manifest for the platform
	for _, m := range manifest.Manifests {
		if m.Platform == nil {
			continue
		}
		if m.Platform.OS == platform.OS && m.Platform.Architecture == platform.Arch && m.Platform.Variant == platform.Variant {
			return m.Digest, nil
		}
	}

	return v1.Hash{}, fmt.Errorf("no manifest found for platform %s", platform)
}

func verifyImagePlatform(img v1.Image, platform Platform) error {
	cfg, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("failed to read base image config: %w", err)
	}
	imagePlatform := Platform{OS: cfg.OS, Arch: cfg.Architecture, Variant: cfg.Variant}
	if imagePlatform != platform {
		return fmt.Errorf("base image platform mismatch: image is %s, requested %s", imagePlatform, platform)
	}
	return nil
}
