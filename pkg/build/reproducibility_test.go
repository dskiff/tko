package build

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

func newTestBuildContext(t *testing.T) BuildContext {
	t.Helper()
	watcher := NewExitCleanupWatcher()
	t.Cleanup(func() { watcher.Close() })
	return BuildContext{
		Context:            context.Background(),
		ExitCleanupWatcher: watcher,
		Keychain:           authn.DefaultKeychain,
		TempPath:           t.TempDir(),
	}
}

func createTestSourceDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for relPath, content := range files {
		absPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newScratchBuildSpec(sourcePath string) BuildSpec {
	return BuildSpec{
		BaseRef: "scratch",
		InjectLayer: BuildSpecInjectLayer{
			Platform:         Platform{OS: "linux", Arch: "amd64"},
			SourcePath:       sourcePath,
			DestinationPath:  "/app",
			DestinationChown: true,
			Entrypoint:       "/app/mybin",
		},
		Author:      "tko-test",
		Annotations: map[string]string{"org.opencontainers.image.version": "1.0.0"},
		Env:         map[string]string{"FOO": "bar"},
	}
}

func TestReproducibleBuild_SameInputsSameDigest(t *testing.T) {
	ctx := newTestBuildContext(t)
	srcDir := createTestSourceDir(t, map[string]string{
		"mybin":      "#!/bin/sh\necho hello\n",
		"config.yml": "key: value\n",
	})
	spec := newScratchBuildSpec(srcDir)

	img1, err := buildImage(ctx, spec)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	img2, err := buildImage(ctx, spec)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}

	d1, err := img1.Digest()
	if err != nil {
		t.Fatalf("failed to get digest 1: %v", err)
	}
	d2, err := img2.Digest()
	if err != nil {
		t.Fatalf("failed to get digest 2: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("digests differ: %s vs %s", d1, d2)
	}

	raw1, err := img1.RawManifest()
	if err != nil {
		t.Fatalf("failed to get raw manifest 1: %v", err)
	}
	raw2, err := img2.RawManifest()
	if err != nil {
		t.Fatalf("failed to get raw manifest 2: %v", err)
	}
	if string(raw1) != string(raw2) {
		t.Fatalf("raw manifests differ")
	}
}

func TestReproducibleBuild_TarLayerTimestamps(t *testing.T) {
	ctx := newTestBuildContext(t)
	srcDir := createTestSourceDir(t, map[string]string{
		"bin/mybin":  "binary content",
		"config.yml": "key: value",
	})

	tarPath, err := createTarFromFolder(ctx, srcDir, "/app", true)
	if err != nil {
		t.Fatalf("createTarFromFolder failed: %v", err)
	}

	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("failed to open tar: %v", err)
	}
	defer f.Close()

	reader := tar.NewReader(f)
	entryCount := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}
		entryCount++

		if !header.AccessTime.Equal(unixEpoch) {
			t.Fatalf("entry %q: AccessTime = %v, want unix epoch", header.Name, header.AccessTime)
		}
		if !header.ChangeTime.Equal(unixEpoch) {
			t.Fatalf("entry %q: ChangeTime = %v, want unix epoch", header.Name, header.ChangeTime)
		}
		if !header.ModTime.Equal(unixEpoch) {
			t.Fatalf("entry %q: ModTime = %v, want unix epoch", header.Name, header.ModTime)
		}
		if header.PAXRecords != nil {
			t.Fatalf("entry %q: PAXRecords = %v, want nil", header.Name, header.PAXRecords)
		}
		if header.Xattrs != nil {
			t.Fatalf("entry %q: Xattrs = %v, want nil", header.Name, header.Xattrs)
		}
		if header.Uid != 0 || header.Gid != 0 {
			t.Fatalf("entry %q: Uid=%d Gid=%d, want 0/0", header.Name, header.Uid, header.Gid)
		}
		if header.Uname != "root" || header.Gname != "root" {
			t.Fatalf("entry %q: Uname=%q Gname=%q, want root/root", header.Name, header.Uname, header.Gname)
		}
	}
	if entryCount == 0 {
		t.Fatal("tar contained no entries")
	}
}

func TestReproducibleBuild_ImageConfig(t *testing.T) {
	ctx := newTestBuildContext(t)
	srcDir := createTestSourceDir(t, map[string]string{
		"mybin": "binary",
	})
	spec := newScratchBuildSpec(srcDir)

	img, err := buildImage(ctx, spec)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("failed to get config: %v", err)
	}

	if !cfg.Created.Time.Equal(unixEpoch) {
		t.Fatalf("Created = %v, want unix epoch", cfg.Created.Time)
	}
	if cfg.Container != "" {
		t.Fatalf("Container = %q, want empty", cfg.Container)
	}
	if cfg.DockerVersion != "" {
		t.Fatalf("DockerVersion = %q, want empty", cfg.DockerVersion)
	}
	if cfg.Author != "tko-test" {
		t.Fatalf("Author = %q, want %q", cfg.Author, "tko-test")
	}
	if len(cfg.Config.Entrypoint) != 1 || cfg.Config.Entrypoint[0] != "/app/mybin" {
		t.Fatalf("Entrypoint = %v, want [/app/mybin]", cfg.Config.Entrypoint)
	}
	if cfg.Config.WorkingDir != "/app" {
		t.Fatalf("WorkingDir = %q, want /app", cfg.Config.WorkingDir)
	}
	if cfg.Config.Labels["org.opencontainers.image.base.name"] != "scratch" {
		t.Fatalf("base.name label = %q, want scratch", cfg.Config.Labels["org.opencontainers.image.base.name"])
	}

	foundEnv := false
	for _, e := range cfg.Config.Env {
		if e == "FOO=bar" {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Fatalf("expected FOO=bar in env, got %v", cfg.Config.Env)
	}
}

func TestReproducibleBuild_HistoryTimestamps(t *testing.T) {
	ctx := newTestBuildContext(t)
	srcDir := createTestSourceDir(t, map[string]string{
		"mybin": "binary",
	})
	spec := newScratchBuildSpec(srcDir)

	img, err := buildImage(ctx, spec)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("failed to get config: %v", err)
	}

	if len(cfg.History) == 0 {
		t.Fatal("expected at least one history entry")
	}

	last := cfg.History[len(cfg.History)-1]
	if !last.Created.Time.Equal(unixEpoch) {
		t.Fatalf("history Created = %v, want unix epoch", last.Created.Time)
	}
	if last.CreatedBy != "tko build" {
		t.Fatalf("history CreatedBy = %q, want %q", last.CreatedBy, "tko build")
	}
}

func TestReproducibleBuild_DifferentInputsDifferentDigest(t *testing.T) {
	ctx := newTestBuildContext(t)

	srcDir1 := createTestSourceDir(t, map[string]string{"mybin": "version-1"})
	srcDir2 := createTestSourceDir(t, map[string]string{"mybin": "version-2"})

	spec1 := newScratchBuildSpec(srcDir1)
	spec2 := newScratchBuildSpec(srcDir2)

	img1, err := buildImage(ctx, spec1)
	if err != nil {
		t.Fatalf("build 1 failed: %v", err)
	}
	img2, err := buildImage(ctx, spec2)
	if err != nil {
		t.Fatalf("build 2 failed: %v", err)
	}

	d1, err := img1.Digest()
	if err != nil {
		t.Fatalf("digest 1 failed: %v", err)
	}
	d2, err := img2.Digest()
	if err != nil {
		t.Fatalf("digest 2 failed: %v", err)
	}
	if d1 == d2 {
		t.Fatalf("expected different digests for different inputs, both got %s", d1)
	}
}

func TestReproducibleBuild_WithAnnotationsAndEnv(t *testing.T) {
	ctx := newTestBuildContext(t)
	srcDir := createTestSourceDir(t, map[string]string{"mybin": "binary"})

	spec := newScratchBuildSpec(srcDir)
	spec.Annotations = map[string]string{
		"org.opencontainers.image.version": "2.0.0",
		"org.opencontainers.image.title":   "test-app",
		"org.opencontainers.image.url":     "https://example.com",
		"org.opencontainers.image.source":  "https://example.com/source",
		"org.opencontainers.image.vendor":  "test-vendor",
	}
	spec.Env = map[string]string{
		"APP_ENV":  "production",
		"APP_PORT": "8080",
		"APP_HOST": "0.0.0.0",
		"LOG_LEVEL": "info",
		"DEBUG":    "false",
	}

	// Build multiple times to catch non-deterministic map iteration
	var firstDigest string
	for i := 0; i < 5; i++ {
		img, err := buildImage(ctx, spec)
		if err != nil {
			t.Fatalf("build %d failed: %v", i, err)
		}
		d, err := img.Digest()
		if err != nil {
			t.Fatalf("digest %d failed: %v", i, err)
		}
		if i == 0 {
			firstDigest = d.String()
		} else if d.String() != firstDigest {
			t.Fatalf("build %d digest %s differs from first build %s", i, d, firstDigest)
		}
	}
}

func TestReproducibleBuild_MultipleFilesAndDirs(t *testing.T) {
	ctx := newTestBuildContext(t)
	srcDir := createTestSourceDir(t, map[string]string{
		"bin/mybin":           "#!/bin/sh\necho hello",
		"config/app.yml":      "app:\n  port: 8080\n",
		"config/logging.yml":  "level: info\n",
		"static/index.html":   "<html><body>hello</body></html>",
	})
	spec := newScratchBuildSpec(srcDir)

	img1, err := buildImage(ctx, spec)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	img2, err := buildImage(ctx, spec)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}

	d1, err := img1.Digest()
	if err != nil {
		t.Fatalf("digest 1 failed: %v", err)
	}
	d2, err := img2.Digest()
	if err != nil {
		t.Fatalf("digest 2 failed: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("digests differ with nested dirs: %s vs %s", d1, d2)
	}

	// Verify correct number of layers
	layers, err := img1.Layers()
	if err != nil {
		t.Fatalf("failed to get layers: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
}
