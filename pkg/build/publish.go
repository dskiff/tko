package build

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func publish(ctx BuildContext, image v1.Image, target BuildSpecTarget) error {
	tag, err := name.NewTag(target.Repo)
	if err != nil {
		return fmt.Errorf("failed to parse target repo: %w", err)
	}

	digest, err := image.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve new image digest: %w", err)
	}

	log.Printf("Publishing %s...", tag.Name()+"@"+digest.String())

	switch target.Type {
	case REMOTE:
		log.Println("Publishing to remote...")

		err := remote.Write(tag, image, remote.WithContext(ctx.Context),
			remote.WithAuthFromKeychain(ctx.Keychain))
		if err != nil {
			return fmt.Errorf("failed to write image to remote: %w", err)
		}
	case LOCAL_DAEMON:
		log.Println("Publishing to local daemon...")
		_, err := daemon.Write(tag, image, daemon.WithContext(ctx.Context))
		if err != nil {
			return fmt.Errorf("failed to write image to daemon: %w", err)
		}
	default:
		return fmt.Errorf("unknown target type: %d", target.Type)
	}

	return nil
}
