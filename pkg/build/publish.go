package build

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func Publish(ctx context.Context, tag name.Tag, image v1.Image, cfg RunConfig) error {
	digest, err := image.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve new image digest: %w", err)
	}

	log.Printf("Publishing %s...", tag.Name()+"@"+digest.String())

	switch cfg.TargetType {
	case REMOTE:
		log.Println("Publishing to remote...")

		keychain := authn.NewMultiKeychain(
			authn.DefaultKeychain,
			google.Keychain,
			github.Keychain,
		)

		err := remote.Write(tag, image, remote.WithContext(ctx),
			remote.WithAuthFromKeychain(keychain))
		if err != nil {
			return fmt.Errorf("failed to write image to remote: %w", err)
		}
	case LOCAL_DAEMON:
		log.Println("Publishing to local daemon...")
		_, err := daemon.Write(tag, image, daemon.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to write image to daemon: %w", err)
		}
	default:
		return fmt.Errorf("unknown target type: %d", cfg.TargetType)
	}

	return nil
}
