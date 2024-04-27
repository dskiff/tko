package cmd

import (
	"context"
	"fmt"

	"github.com/dskiff/tko/pkg/build"
)

type CliCtx struct {
	Context          context.Context
	TkoBuildVersion  string
	TkoBuildCommit   string
	TkoBuildDate     string
	ExitCleanWatcher *build.ExitCleanupWatcher
}

type VersionCmd struct{}

func (v *VersionCmd) Run(cliCtx *CliCtx) error {
	fmt.Println(cliCtx.TkoBuildVersion)
	return nil
}

type CLI struct {
	Version VersionCmd `cmd:"" help:"Show version."`

	Build BuildCmd `cmd:"" help:"Build and publish a container image."`
}
