package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/dskiff/tko/pkg/build"

	"github.com/joho/godotenv"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	godotenv.Load(".env.local")
	godotenv.Load(".env")

	TKO_TARGET_REPO := os.Getenv("TKO_TARGET_REPO")
	TKO_BASE_IMAGE := os.Getenv("TKO_BASE_IMAGE")
	TKO_DEST_PATH := os.Getenv("TKO_DEST_PATH")
	TKO_ENTRYPOINT := os.Getenv("TKO_ENTRYPOINT")
	TKO_TEMP_PATH := os.Getenv("TKO_TEMP_PATH")
	TKO_TARGET_TYPE := os.Getenv("TKO_TARGET_TYPE")

	if TKO_BASE_IMAGE == "" {
		TKO_BASE_IMAGE = "cgr.dev/chainguard/static:latest"
	}
	if TKO_DEST_PATH == "" {
		TKO_DEST_PATH = "/tko-app"
	}
	if TKO_ENTRYPOINT == "" {
		TKO_ENTRYPOINT = "/tko-app/app"
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

	// TODO: Better command line parsing
	if len(os.Args) < 2 {
		log.Fatalln("source path is required. Usage: tko <source-path>")
	}
	srcPath := os.Args[1]
	if srcPath == "" {
		log.Fatalln("source path is required. Usage: tko <source-path>")
	}

	log.Println("TKO_TARGET_REPO:", TKO_TARGET_REPO)
	log.Println("TKO_BASE_IMAGE:", TKO_BASE_IMAGE)
	log.Println("TKO_DEST_PATH:", TKO_DEST_PATH)
	log.Println("TKO_ENTRYPOINT:", TKO_ENTRYPOINT)
	log.Println("TKO_TEMP_PATH:", TKO_TEMP_PATH)
	log.Println("Source path:", srcPath)
	log.Println("")

	exitCleanWatcher := build.NewExitCleanupWatcher()
	defer exitCleanWatcher.Close()

	err := build.Run(ctx, build.RunConfig{
		SrcPath:    srcPath,
		DstPath:    TKO_DEST_PATH,
		Entrypoint: TKO_ENTRYPOINT,

		BaseImage:  TKO_BASE_IMAGE,
		TargetRepo: TKO_TARGET_REPO,
		TargetType: targetType,

		PlatformOs:   "linux",
		PlatformArch: "amd64",

		TempPath:           TKO_TEMP_PATH,
		ExitCleanupWatcher: exitCleanWatcher,
	})
	if err != nil {
		log.Println(err)
		exitCleanWatcher.Close()
		os.Exit(1)
	}
}
