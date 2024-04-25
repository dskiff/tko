package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/dskiff/tko/pkg/build"
	"github.com/dskiff/tko/pkg/cmd"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
)

var version = "dev"
var commit = "none"
var date = "unknown"

func main() {
	log.Printf("tko %s (%s) built on %s\n", version, commit, date)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	godotenv.Load(".env.local")
	godotenv.Load(".env")

	exitCleanWatcher := build.NewExitCleanupWatcher()
	defer exitCleanWatcher.Close()

	cliContext := cmd.CliCtx{
		Ctx:              &ctx,
		Version:          version,
		ExitCleanWatcher: exitCleanWatcher,
	}

	args := kong.Parse(&cmd.CLI)

	err := args.Run(&cliContext)
	if err != nil {
		log.Println(err)
		exitCleanWatcher.Close()
		os.Exit(1)
	}
}
