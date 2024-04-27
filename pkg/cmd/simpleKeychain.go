package cmd

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type SimpleKeychain struct {
	Username string
	Password string
	Registry string
}

func newSimpleKeychain(username, password, targetRepo string) (SimpleKeychain, error) {
	// Resolve the name first (should auto-inject default docker, for example)
	ref, err := name.ParseReference(targetRepo)
	if err != nil {
		return SimpleKeychain{}, err
	}

	uri := ref.Context().Name()
	if !strings.Contains(uri, "://") {
		uri = "https://" + uri
	}

	// get domain from target repo
	url, err := url.Parse(uri)
	if err != nil {
		return SimpleKeychain{}, err
	}

	return SimpleKeychain{
		Username: username,
		Password: password,
		Registry: url.Hostname(),
	}, nil
}

func (s *SimpleKeychain) toKeychain() authn.Keychain {
	return authn.NewKeychainFromHelper(s)
}

func (s *SimpleKeychain) Get(serverURL string) (string, string, error) {
	// if the serverURL is not the same as the registry, return an error
	if serverURL != s.Registry {
		log.Printf("Not using provided credentials for %s because it does not match target registry %s", serverURL, s.Registry)
		return "", "", fmt.Errorf("serverURL %s does not match registry %s", serverURL, s.Registry)
	}

	log.Println("Using provided credentials for", s.Registry)
	return s.Username, s.Password, nil
}
