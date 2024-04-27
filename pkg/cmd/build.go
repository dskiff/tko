package cmd

import (
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
	BaseRef string `short:"b" help:"Base image reference" env:"TKO_BASE_IMAGE" default:"ubuntu:jammy@latest"`

	Platform string `short:"p" help:"Platform to build for" env:"TKO_PLATFORM" default:"linux/amd64"`

	SourcePath      string `arg:"" name:"path" help:"Path to artifacts to embed" type:"path" env:"TKO_SOURCE_PATH"`
	DestinationPath string `short:"d" help:"Path to embed artifacts in" env:"TKO_DEST_PATH" default:"/tko-app"`
	Entrypoint      string `short:"e" help:"Entrypoint for the embedded artifacts" env:"TKO_ENTRYPOINT" default:"/tko-app/app"`

	TargetRepo string `short:"t" help:"Target repository" env:"TKO_TARGET_REPO" required:"true"`
	TargetType string `short:"T" help:"Target type" env:"TKO_TARGET_TYPE" default:"REMOTE" enum:"REMOTE,LOCAL_DAEMON"`

	Verbose bool `short:"v" help:"Enable verbose output"`
}

func (b *BuildCmd) Run(cliCtx *CliCtx) error {
	targetType, err := build.ParseTargetType(b.TargetType)
	if err != nil {
		return err
	}

	platform, err := build.ParsePlatform(b.Platform)
	if err != nil {
		return err
	}

	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
	)

	cfg := build.BuildSpec{
		BaseRef: b.BaseRef,
		InjectLayer: build.BuildSpecInjectLayer{
			Platform:        platform,
			SourcePath:      b.SourcePath,
			DestinationPath: b.DestinationPath,
			Entrypoint:      b.Entrypoint,
		},
		Target: build.BuildSpecTarget{
			Repo: b.TargetRepo,
			Type: targetType,
		},
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

	return build.Build(build.BuildContext{
		Context:            cliCtx.Context,
		ExitCleanupWatcher: cliCtx.ExitCleanWatcher,
		Keychain:           keychain,

		TempPath: os.Getenv("TKO_TEMP_PATH"),
	}, cfg)
}
