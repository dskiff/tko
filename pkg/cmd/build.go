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

	cfg := build.BuildSpec{
		BaseRef: os.Getenv("TKO_BASE_IMAGE"),
		InjectLayer: build.BuildSpecInjectLayer{
			Platform: build.Platform{
				OS:   "linux",
				Arch: "amd64",
			},
			SourcePath:      b.SourcePath,
			DestinationPath: os.Getenv("TKO_DEST_PATH"),
			Entrypoint:      os.Getenv("TKO_ENTRYPOINT"),
		},
		Target: build.BuildSpecTarget{
			Repo: os.Getenv("TKO_TARGET_REPO"),
			Type: targetType,
		},
	}

	if cfg.BaseRef == "" {
		cfg.BaseRef = "cgr.dev/chainguard/static:latest"
	}

	if cfg.Target.Repo == "" {
		return fmt.Errorf("target repo must be set")
	}

	if cfg.InjectLayer.DestinationPath == "" {
		cfg.InjectLayer.DestinationPath = "/tko-app"
	}

	if cfg.InjectLayer.Entrypoint == "" {
		cfg.InjectLayer.Entrypoint = "/tko-app/app"
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
		Ctx:                cliCtx.Ctx,
		ExitCleanupWatcher: cliCtx.ExitCleanWatcher,
		Keychain:           keychain,

		TempPath: os.Getenv("TKO_TEMP_PATH"),
	}, cfg)
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
