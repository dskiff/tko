package main_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	"github.com/dskiff/tko/pkg/cmd"
	"gotest.tools/v3/assert"
)

func TestYamlBasic(t *testing.T) {
	yaml := `
build:
  base-ref: base-image@sha256:1234

  platform: custom-os/arch-variant

  entrypoint: /entrypoint
  destination-path: /destination
  target-repo: repo/target

  author: me
  labels:
    label1: value1
    label2: value2
  
  tmp: /tmp-dir
  verbose: true
`

	r, err := kongyaml.Loader(strings.NewReader(yaml))
	assert.NilError(t, err)

	cli := cmd.CLI{}
	parser := mustNew(t, &cli, kong.Resolvers(r))
	_, err = parser.Parse([]string{"build", "/source"})
	assert.NilError(t, err)

	assert.Equal(t, "base-image@sha256:1234", cli.Build.BaseRef)

	assert.Equal(t, "custom-os/arch-variant", cli.Build.Platform)

	assert.Equal(t, "/entrypoint", cli.Build.Entrypoint)
	assert.Equal(t, "/destination", cli.Build.DestinationPath)
	assert.Equal(t, "repo/target", cli.Build.TargetRepo)

	assert.Equal(t, "me", cli.Build.Author)
	assert.Equal(t, "value1", cli.Build.Labels["label1"])
	assert.Equal(t, "value2", cli.Build.Labels["label2"])

	assert.Equal(t, "/tmp-dir", cli.Build.Tmp)
	assert.Equal(t, true, cli.Build.Verbose)
}

func TestVersionArgs(t *testing.T) {
	cli := cmd.CLI{}
	parser := mustNew(t, &cli)
	args, err := parser.Parse([]string{"version"})
	assert.NilError(t, err)

	assert.Equal(t, true, args.Command() == "version")
}

func TestBuildArgs(t *testing.T) {
	cli := cmd.CLI{}
	parser := mustNew(t, &cli)
	_, err := parser.Parse([]string{"build", "/source",
		"-b", "base-image@sha256:1234",
		"-p", "custom-os/arch-variant",
		"-e", "/entrypoint",
		"-d", "/destination",
		"-t", "repo/target",
		"-a", "me",
		"-l", "label1=value1",
		"-l", "label2=value2",
		"-T", "REMOTE",
		"-v",
		"--tmp", "/tmp-dir",
	})
	assert.NilError(t, err)

	assert.Equal(t, "base-image@sha256:1234", cli.Build.BaseRef)

	assert.Equal(t, "custom-os/arch-variant", cli.Build.Platform)

	assert.Equal(t, "/entrypoint", cli.Build.Entrypoint)
	assert.Equal(t, "/destination", cli.Build.DestinationPath)
	assert.Equal(t, "repo/target", cli.Build.TargetRepo)

	assert.Equal(t, "me", cli.Build.Author)
	assert.Equal(t, "value1", cli.Build.Labels["label1"])
	assert.Equal(t, "value2", cli.Build.Labels["label2"])

	assert.Equal(t, "/tmp-dir", cli.Build.Tmp)
	assert.Equal(t, true, cli.Build.Verbose)
}

func mustNew(t *testing.T, cli interface{}, options ...kong.Option) *kong.Kong {
	t.Helper()
	options = append([]kong.Option{
		kong.Name("test"),
		kong.Exit(func(int) {
			t.Helper()
			t.Fatalf("unexpected exit()")
		}),
	}, options...)
	parser, err := kong.New(cli, options...)
	assert.NilError(t, err)
	return parser
}
