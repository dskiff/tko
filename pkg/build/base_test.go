package build

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// mockIndex implements v1.ImageIndex for testing getDigestForPlatform.
type mockIndex struct {
	manifests []v1.Descriptor
}

func (m *mockIndex) MediaType() (types.MediaType, error) {
	return types.DockerManifestList, nil
}

func (m *mockIndex) Digest() (v1.Hash, error) {
	return v1.Hash{}, nil
}

func (m *mockIndex) Size() (int64, error) {
	return 0, nil
}

func (m *mockIndex) IndexManifest() (*v1.IndexManifest, error) {
	return &v1.IndexManifest{Manifests: m.manifests}, nil
}

func (m *mockIndex) RawManifest() ([]byte, error) {
	return nil, nil
}

func (m *mockIndex) Image(v1.Hash) (v1.Image, error) {
	return nil, nil
}

func (m *mockIndex) ImageIndex(v1.Hash) (v1.ImageIndex, error) {
	return nil, nil
}

func TestGetDigestForPlatformNilPlatform(t *testing.T) {
	amd64Hash := v1.Hash{Algorithm: "sha256", Hex: "amd64"}
	idx := &mockIndex{
		manifests: []v1.Descriptor{
			{Digest: v1.Hash{Algorithm: "sha256", Hex: "nilplat"}, Platform: nil},
			{Digest: amd64Hash, Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}},
		},
	}

	hash, err := getDigestForPlatform(idx, Platform{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != amd64Hash {
		t.Fatalf("expected %v, got %v", amd64Hash, hash)
	}
}

func TestGetDigestForPlatformVariantMatch(t *testing.T) {
	v7Hash := v1.Hash{Algorithm: "sha256", Hex: "armv7"}
	v6Hash := v1.Hash{Algorithm: "sha256", Hex: "armv6"}
	idx := &mockIndex{
		manifests: []v1.Descriptor{
			{Digest: v6Hash, Platform: &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v6"}},
			{Digest: v7Hash, Platform: &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}},
		},
	}

	hash, err := getDigestForPlatform(idx, Platform{OS: "linux", Arch: "arm", Variant: "v7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != v7Hash {
		t.Fatalf("expected v7 hash, got %v", hash)
	}
}

func TestGetDigestForPlatformVariantMismatch(t *testing.T) {
	idx := &mockIndex{
		manifests: []v1.Descriptor{
			{Digest: v1.Hash{Algorithm: "sha256", Hex: "armv6"}, Platform: &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v6"}},
		},
	}

	_, err := getDigestForPlatform(idx, Platform{OS: "linux", Arch: "arm", Variant: "v7"})
	if err == nil {
		t.Fatal("expected error for variant mismatch")
	}
}
