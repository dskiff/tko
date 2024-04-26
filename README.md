# tko

tko builds OCI images without elevated privileges. 

No `privileged`.
No DinD.
No root or sudo.
drop ALL `capabilities` to your hearts content.

```
# Perform build, output to ./dist
# Provide credentials using docker config, GITHUB_TOKEN, etc

tko build --target-repo="destination/repo" ./dist
```

## What?

tko is:
- Simple (pull base image, add content, push to registry)
- Low footprint (<4MiB, single static binary, no runtime deps)
- Rootless (no sudo/daemon/chroot/caps/goats blood/etc needed)
- Reproducible (same build artifacts -> same image digest)

tko is NOT a replacement for generic docker build (or buildah, kaniko, etc). It cannot run a Dockerfile. It combines your build artifacts with a base image and modifies metadata. That's it. For me, this was enough for the majority of my container builds, but YMMV.
 
## Why?

Constructing containers inside of a constrained environment stinks (e.g. a k8s pod with a reasonable PSA). While there are a number of existing solutions, they all have substantial tradeoffs.

- DinD has considerable security implications, and depending on your environment may be a non-starter.
- DinK avoids exposing your daemon directly, but introduces some fun new security issues. Additionally, it adds more moving pieces and has some resource/performance issues (e.g. RWX PVCs).
- I have never gotten [buildah](https://github.com/containers/buildah) to work well in constrained environments (e.g. requiring something like `CAP_SETUID`). That being said, both it and kaniko _do more_ than tko. Much more. 
- [kaniko](https://github.com/GoogleContainerTools/kaniko) only supports being run in the published container. I've actually gotten the most mileage with kaniko in constrained environments (other than ko), but it often came with hacks or quirks to make it work how I wanted it to.

The complexity and tradeoffs are there to support the complex behavior that `Dockerfile`s can exhibit. However, in the 80% case, I found I did not need that level of flexibility.

Enter ko. ko is a simple, single binary. You call `ko build`. It doesn't require any privileges. It builds your app, packages it in a container, and ships it to your repo. Done.

Unfortunately, ko is only for go. If you're using go, and by some weird SEO quirk you ended up here instead. Stop. Go use [ko](https://ko.build). If you're not, tko may be your answer.

## Examples

### Quarkus + Rootless Github Self Hosted Runners

```
- uses: graalvm/setup-graalvm@2f25c0caae5b220866f732832d5e3e29ff493338
  with:
    java-version: '17'
    distribution: 'mandrel'

- use: dskiff/setup-tko@main
    
- run: |
    ./mvnw package -Dnative
    mkdir -p out
    mv target/*-runner out/app

- run: tko build "./out"
  env:
    TKO_BASE_IMAGE: ubuntu:jammy@sha256:6d7b5d3317a71adb5e175640150e44b8b9a9401a7dd394f44840626aff9fa94d
    GITHUB_TOKEN: ${{ github.token }}
```

### Deno + Rootless Github Self Hosted Runners

```
- uses: denoland/setup-deno@v1
  with:
    deno-version: v1.x

- use: dskiff/setup-tko@main

- run: deno compile --lock=deno.lock --output dist/app src/main.ts 

- run: tko build "./dist"
  env:
    TKO_BASE_IMAGE: ubuntu:jammy@sha256:6d7b5d3317a71adb5e175640150e44b8b9a9401a7dd394f44840626aff9fa94d
    GITHUB_TOKEN: ${{ github.token }}
```

## Other Options

Aside from kaniko and buildah, there are a number of other tools you might find useful instead. I'm sure I'm missing some, but:

- [umoci](https://umo.ci/) and [crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md) are CLIs for interacting with OCI images. You can accomplish a lot with these and a shell script, but I wanted something simpler and easier to maintain.
- [stacker](https://github.com/project-stacker/stacker). I have not used it and can't vouch for it, but it seems to live in a middle ground between tko and something more like kaniko. In my case, its added complexity did not seem worth it, but it may be worth checking out if you're looking in this space.
- [apko](https://github.com/chainguard-dev/apko) + [melange](https://github.com/chainguard-dev/melange). The tooling story is pretty rough at the time of writing this, but I like the direction. If you're an enterprise, looking to do enterprise-y things, I would recommend checking them out.
