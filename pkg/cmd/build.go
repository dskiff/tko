package cmd

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/dskiff/tko/pkg/build"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/google"
)

type BuildCmd struct {
	SourcePath string `arg:"" name:"path" help:"Path to artifacts to embed" type:"path"`

	Verbose bool `short:"v" help:"Enable verbose output"`
}

func (b *BuildCmd) Run(cliCtx *CliCtx) error {
	targetType, err := parseTargetType(os.Getenv("TKO_TARGET_TYPE"))
	if err != nil {
		return err
	}

	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
	)

	cfg := build.RunConfig{
		SrcPath:    b.SourcePath,
		DstPath:    os.Getenv("TKO_DEST_PATH"),
		Entrypoint: os.Getenv("TKO_ENTRYPOINT"),

		BaseImage:      os.Getenv("TKO_BASE_IMAGE"),
		TargetRepo:     os.Getenv("TKO_TARGET_REPO"),
		TargetType:     targetType,
		RemoteKeychain: keychain,

		PlatformOs:   "linux",
		PlatformArch: "amd64",

		TempPath:           os.Getenv("TKO_TEMP_PATH"),
		ExitCleanupWatcher: cliCtx.ExitCleanWatcher,
	}

	if cfg.BaseImage == "" {
		cfg.BaseImage = "cgr.dev/chainguard/static:latest"
	}

	if cfg.TargetRepo == "" {
		return fmt.Errorf("target repo must be set")
	}

	if cfg.DstPath == "" {
		cfg.DstPath = "/tko-app"
	}

	if cfg.Entrypoint == "" {
		cfg.Entrypoint = "/tko-app/app"
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	log.Printf("tko %s (%s) built on %s\n", cliCtx.TkoBuildVersion, cliCtx.TkoBuildCommit, cliCtx.TkoBuildDate)
	log.Println("Build configuration:", "\n"+string(out))

	// Enable go-containerregistry logging
	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)
	if b.Verbose {
		logs.Debug.SetOutput(os.Stderr)
	}

	return build.Run(cliCtx.Ctx, cfg)
}

func parseTargetType(str string) (build.TargetType, error) {
	switch str {
	case "REMOTE":
		return build.REMOTE, nil
	case "LOCAL_DAEMON":
		return build.LOCAL_DAEMON, nil
	case "":
		return build.REMOTE, nil
	default:
		return -1, fmt.Errorf("invalid target type: %s", str)
	}
}
