package cmd

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/dskiff/tko/pkg/build"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/google"
)

type BuildCmd struct {
	BaseRef string `short:"b" help:"Base image reference" env:"TKO_BASE_REF" default:"ubuntu:jammy"`

	Platform string `short:"p" help:"Platform to build for" env:"TKO_PLATFORM" default:"linux/amd64"`

	SourcePath       string `arg:"" help:"Path to artifacts to embed" type:"path" env:"TKO_SOURCE_PATH"`
	DestinationPath  string `short:"d" help:"Path to embed artifacts in" env:"TKO_DEST_PATH" default:"/tko-app"`
	DestinationChown bool   `help:"Whether to chown the destination path to root:root" default:"true"`
	Entrypoint       string `short:"e" help:"Entrypoint for the embedded artifacts" env:"TKO_ENTRYPOINT" default:"/tko-app/app"`

	TargetRepo string `short:"t" help:"Target repository" env:"TKO_TARGET_REPO" required:"true"`
	TargetType string `short:"T" help:"Target type" env:"TKO_TARGET_TYPE" default:"REMOTE" enum:"REMOTE,LOCAL_DAEMON,LOCAL_FILE"`

	Author                string            `help:"Author of the build" env:"TKO_AUTHOR" default:"github.com/dskiff/tko"`
	DefaultAnnotations    map[string]string `short:"A" help:"Default annotations to apply to the image" env:"TKO_DEFAULT_ANNOTATIONS" default:"" mapsep:"," sep:"="`
	Annotations           map[string]string `short:"a" help:"Additional annotations to apply to the image. Can override default-annotations." env:"TKO_ANNOTATIONS" default:"" mapsep:"," sep:"="`
	AutoVersionAnnotation string            `help:"Automatically version annotations" env:"TKO_AUTO_VERSION_ANNOTATION" default:"none" enum:"git,none"`

	RegistryUser string `help:"Registry user. Used for target registry url. You can use standard docker config for more complex auth." env:"TKO_REGISTRY_USER"`
	RegistryPass string `help:"Registry password. Used for target registry url. You can use standard docker config for more complex auth." env:"TKO_REGISTRY_PASS"`

	Tmp     string `help:"Path where tko can write temporary files. Defaults to golang's tmp logic." env:"TKO_TMP" default:""`
	Verbose bool   `short:"v" help:"Enable verbose output"`
}

func (b *BuildCmd) Run(cliCtx *CliCtx) error {
	log.Printf("tko %s (%s) built on %s\n", cliCtx.TkoBuildVersion, cliCtx.TkoBuildCommit, cliCtx.TkoBuildDate)

	targetType, err := build.ParseTargetType(b.TargetType)
	if err != nil {
		return err
	}

	platform, err := build.ParsePlatform(b.Platform)
	if err != nil {
		return err
	}

	keychains := []authn.Keychain{
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
	}

	if b.RegistryUser != "" && b.RegistryPass != "" {
		k, err := newSimpleKeychain(b.RegistryUser, b.RegistryPass, b.TargetRepo)
		if err != nil {
			return fmt.Errorf("failed to create keychain: %w", err)
		}

		keychains = append([]authn.Keychain{k.toKeychain()}, keychains...)
	}
	keychain := authn.NewMultiKeychain(keychains...)

	annotations := make(map[string]string)
	if b.AutoVersionAnnotation == "git" {
		gitInfo, err := getGitInfo(b.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to get git info: %w", err)
		}

		gitInfoStr, err := yaml.Marshal(gitInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal git info: %w", err)
		}
		log.Print("Found git info:", "\n"+string(gitInfoStr))

		if len(gitInfo.Tag) > 1 {
			return fmt.Errorf("multiple tags found for commit %s: %v", gitInfo.CommitHash, gitInfo.Tag)
		}

		revision := gitInfo.CommitHash
		if gitInfo.Dirty {
			revision += "-dirty"
		}

		gitVersion := "snapshot-" + gitInfo.CommitHash
		if len(gitInfo.Tag) == 1 {
			gitVersion = gitInfo.Tag[0]
			isValid := regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(gitVersion)
			if !isValid {
				return fmt.Errorf("tag %s is not a valid version", gitVersion)
			}
		}
		if gitInfo.Dirty {
			gitVersion += "-dirty"
		}

		annotations["org.opencontainers.image.revision"] = revision
		annotations["org.opencontainers.image.version"] = gitVersion
	}

	// Annotations would ideally be merged by kong, but this works too
	for k, v := range b.DefaultAnnotations {
		annotations[k] = v
	}
	for k, v := range b.Annotations {
		annotations[k] = v
	}

	cfg := build.BuildSpec{
		BaseRef: b.BaseRef,
		InjectLayer: build.BuildSpecInjectLayer{
			Platform:         platform,
			SourcePath:       b.SourcePath,
			DestinationPath:  b.DestinationPath,
			DestinationChown: b.DestinationChown,
			Entrypoint:       b.Entrypoint,
		},
		Target: build.BuildSpecTarget{
			Repo: b.TargetRepo,
			Type: targetType,
		},

		Author:      b.Author,
		Annotations: annotations,
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	log.Print("Build configuration:", "\n"+string(out))

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

		TempPath: b.Tmp,
	}, cfg)
}
