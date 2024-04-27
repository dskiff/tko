package build

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func publish(ctx BuildContext, image v1.Image, target BuildSpecTarget) error {
	ref, err := name.NewTag(target.Repo)
	if err != nil {
		return fmt.Errorf("failed to parse target repo: %w", err)
	}

	switch target.Type {
	case REMOTE:
		log.Println("Publishing to remote...")

		err := remote.Write(ref, image, remote.WithContext(ctx.Context),
			remote.WithAuthFromKeychain(ctx.Keychain))
		if err != nil {
			return fmt.Errorf("failed to write image to remote: %w", err)
		}
	case LOCAL_DAEMON:
		log.Println("Publishing to local daemon...")
		_, err := daemon.Write(ref, image, daemon.WithContext(ctx.Context))
		if err != nil {
			return fmt.Errorf("failed to write image to daemon: %w", err)
		}
	case LOCAL_FILE:
		log.Println("Publishing to local file...")
		err := tarball.WriteToFile("out.tar", ref, image)
		if err != nil {
			return fmt.Errorf("failed to write image to file: %w", err)
		}
	default:
		return fmt.Errorf("unknown target type: %d", target.Type)
	}

	digest, err := image.Digest()
	if err != nil {
		return fmt.Errorf("failed to retrieve new image digest: %w", err)
	}
	log.Printf("Pushed: %s", ref.Context().Digest(digest.String()))

	return nil
}
