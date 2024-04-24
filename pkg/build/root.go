package build

import (
	"context"
	"errors"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type RunConfig struct {
	SrcPath    string
	DstPath    string
	Entrypoint string

	BaseImage  string
	TargetRepo string

	PlatformOs   string
	PlatformArch string

	TempPath           string
	ExitCleanupWatcher *ExitCleanupWatcher
}

func Run(ctx context.Context, cfg RunConfig) error {
	baseRef, baseIndex, err := fetchImageIndex(ctx, cfg.BaseImage)
	if err != nil {
		log.Fatalln("failed to retrieve base image index:", err)
	}
	baseDigest, err := baseIndex.Digest()
	if err != nil {
		log.Fatalln("failed to retrieve base image digest:", err)
	}
	log.Println("Using base image:", baseRef.Name()+"@"+baseDigest.String())

	baseImage, err := getImageForPlatform(baseIndex, cfg.PlatformArch, cfg.PlatformOs)
	if err != nil {
		log.Fatalln("failed to retrieve base image:", err)
	}

	newLayer, err := createLayerFromFolder(cfg.SrcPath, cfg.DstPath, cfg)
	if err != nil {
		log.Fatalln("failed to create layer from source:", err)
	}

	newImage, err := mutate.AppendLayers(baseImage, newLayer)
	if err != nil {
		log.Fatalln("failed to append layer to base image:", err)
	}

	newImage, err = mutateConfig(newImage, cfg)
	if err != nil {
		log.Fatalln("failed to mutate config:", err)
	}

	newImageDigest, err := newImage.Digest()
	if err != nil {
		log.Fatalln("failed to retrieve new image digest:", err)
	}
	log.Println("Created new image:", newImageDigest)

	if cfg.TargetRepo == "" {
		log.Println("TKO_TARGET_REPO is not set. Skipping publish...")
		return nil
	}

	tag, err := name.NewTag(cfg.TargetRepo)
	if err != nil {
		log.Fatalln("failed to parse target repo:", err)
	}
	result, err := daemon.Write(tag, newImage, daemon.WithContext(ctx))
	if err != nil {
		log.Fatalln("failed to write image to daemon:", err)
	}
	log.Println(result)

	return nil

	// p, err := publish.NewDefault(targetRepo, // publish to example.registry/my-repo
	// 	publish.WithTags([]string{commitSHA}),               // tag with :deadbeef
	// 	publish.WithAuthFromKeychain(authn.DefaultKeychain)) // use credentials from ~/.docker/config.json
	// if err != nil {
	// 	log.Fatalf("NewDefault: %v", err)
	// }
	// ref, err := p.Publish(ctx, r, importpath)
	// if err != nil {
	// 	log.Fatalf("Publish: %v", err)
	// }
	// fmt.Println(ref.String())
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

	return v1.Hash{}, errors.New("platform not found in index")
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
