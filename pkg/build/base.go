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

func getBaseImage(ctx BuildContext, baseRef string, platform Platform, keychain authn.Keychain) (v1.Image, error) {
	if baseRef == "scratch" {
		return empty.Image, nil
	}

	ref, index, err := fetchImageIndex(ctx, baseRef, keychain)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve base image index: %w", err)
	}
	baseDigest, err := index.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve base image digest: %w", err)
	}
	log.Println("Using base image:", ref.Context().Name()+"@"+baseDigest.String())

	return getImageForPlatform(index, platform)
}

func fetchImageIndex(ctx BuildContext, src string, keychain authn.Keychain) (name.Reference, v1.ImageIndex, error) {
	ref, err := name.ParseReference(src)
	if err != nil {
		return nil, nil, err
	}
	base, err := remote.Index(ref, remote.WithContext(ctx.Context), remote.WithAuthFromKeychain(keychain))
	return ref, base, err
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
		if m.Platform.Architecture == platform.Arch && m.Platform.OS == platform.OS {
			return m.Digest, nil
		}
	}

	return v1.Hash{}, fmt.Errorf("no manifest found for platform %s", platform)
}
