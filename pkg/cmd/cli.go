package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dskiff/tko/pkg/build"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/google"
)

type CliCtx struct {
	Ctx              *context.Context
	Version          string
	ExitCleanWatcher *build.ExitCleanupWatcher
}

type VersionCmd struct{}

func (v *VersionCmd) Run(cliCtx *CliCtx) error {
	fmt.Println(cliCtx.Version)
	return nil
}

type BuildCmd struct {
	SourcePath string `arg:"" name:"path" help:"Path to artifacts to embed" type:"path"`
}

func (b *BuildCmd) Run(cliCtx *CliCtx) error {
	TKO_TARGET_REPO := os.Getenv("TKO_TARGET_REPO")
	TKO_BASE_IMAGE := os.Getenv("TKO_BASE_IMAGE")
	TKO_DEST_PATH := os.Getenv("TKO_DEST_PATH")
	TKO_ENTRYPOINT := os.Getenv("TKO_ENTRYPOINT")
	TKO_TEMP_PATH := os.Getenv("TKO_TEMP_PATH")
	TKO_TARGET_TYPE := os.Getenv("TKO_TARGET_TYPE")
	TKO_LOG_LEVEL := os.Getenv("TKO_LOG_LEVEL")

	if TKO_BASE_IMAGE == "" {
		TKO_BASE_IMAGE = "cgr.dev/chainguard/static:latest"
	}
	if TKO_DEST_PATH == "" {
		TKO_DEST_PATH = "/tko-app"
	}
	if TKO_ENTRYPOINT == "" {
		TKO_ENTRYPOINT = "/tko-app/app"
	}
	if TKO_LOG_LEVEL == "" {
		TKO_LOG_LEVEL = "info"
	}

	var targetType build.TargetType
	switch TKO_TARGET_TYPE {
	case "REMOTE":
		targetType = build.REMOTE
	case "LOCAL_DAEMON":
		targetType = build.LOCAL_DAEMON
	case "":
		targetType = build.REMOTE
	default:
		log.Fatalf("Invalid TKO_TARGET_TYPE: %s", TKO_TARGET_TYPE)
	}

	srcPath := b.SourcePath

	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
	)

	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)
	if TKO_LOG_LEVEL == "debug" {
		logs.Debug.SetOutput(os.Stderr)
	}

	log.Println("TKO_TARGET_REPO:", TKO_TARGET_REPO)
	log.Println("TKO_BASE_IMAGE:", TKO_BASE_IMAGE)
	log.Println("TKO_DEST_PATH:", TKO_DEST_PATH)
	log.Println("TKO_ENTRYPOINT:", TKO_ENTRYPOINT)
	log.Println("TKO_TEMP_PATH:", TKO_TEMP_PATH)
	log.Println("TKO_TARGET_TYPE:", TKO_TARGET_TYPE)
	log.Println("TKO_LOG_LEVEL:", TKO_LOG_LEVEL)
	log.Println("Source path:", srcPath)
	log.Println("")

	return build.Run(*cliCtx.Ctx, build.RunConfig{
		SrcPath:    srcPath,
		DstPath:    TKO_DEST_PATH,
		Entrypoint: TKO_ENTRYPOINT,

		BaseImage:      TKO_BASE_IMAGE,
		TargetRepo:     TKO_TARGET_REPO,
		TargetType:     targetType,
		RemoteKeychain: keychain,

		PlatformOs:   "linux",
		PlatformArch: "amd64",

		TempPath:           TKO_TEMP_PATH,
		ExitCleanupWatcher: cliCtx.ExitCleanWatcher,
	})
}

var CLI struct {
	Version VersionCmd `cmd:"" help:"Show version."`

	Build BuildCmd `cmd:"" help:"Build and publish a container image."`
}
