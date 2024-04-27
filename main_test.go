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
  destination-chown: false
  target-repo: repo/target

  author: me
  default-annotations:
    label1: value1
    label2: value2
  annotations:
    label3: value3
    label4: value4
  
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
	assert.Equal(t, false, cli.Build.DestinationChown)
	assert.Equal(t, "repo/target", cli.Build.TargetRepo)

	assert.Equal(t, "me", cli.Build.Author)
	assert.Equal(t, "value1", cli.Build.DefaultAnnotations["label1"])
	assert.Equal(t, "value2", cli.Build.DefaultAnnotations["label2"])
	assert.Equal(t, "value3", cli.Build.Annotations["label3"])
	assert.Equal(t, "value4", cli.Build.Annotations["label4"])

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
		"--destination-chown=false",
		"-t", "repo/target",
		"--author", "me",
		"-A", "label1=value1",
		"-A", "label2=value2",
		"-a", "label3=value3",
		"-a", "label4=value4",
		"-T", "REMOTE",
		"-v",
		"--tmp", "/tmp-dir",
	})
	assert.NilError(t, err)

	assert.Equal(t, "base-image@sha256:1234", cli.Build.BaseRef)

	assert.Equal(t, "custom-os/arch-variant", cli.Build.Platform)

	assert.Equal(t, "/entrypoint", cli.Build.Entrypoint)
	assert.Equal(t, "/destination", cli.Build.DestinationPath)
	assert.Equal(t, false, cli.Build.DestinationChown)
	assert.Equal(t, "repo/target", cli.Build.TargetRepo)

	assert.Equal(t, "me", cli.Build.Author)
	assert.Equal(t, "value1", cli.Build.DefaultAnnotations["label1"])
	assert.Equal(t, "value2", cli.Build.DefaultAnnotations["label2"])
	assert.Equal(t, "value3", cli.Build.Annotations["label3"])
	assert.Equal(t, "value4", cli.Build.Annotations["label4"])

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
