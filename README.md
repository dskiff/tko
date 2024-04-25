# tko

tko is a small, simple tool for building OCI images without needing elevated privileges. No `privileged`. No DinD. No root.

```
# Perform build, output to ./dist
# Provide credentials using docker config, GITHUB_TOKEN, etc

TKO_TARGET_REPO="destination/repo" tko ./dist
```

## What?

tko is:
- Simple (pull base image, add content, push to registry)
- Low footprint (4MiB, single binary)
- "rootless" (it tars the new content and injects it. no daemon/chroot needed).

tko is NOT a replacement for generic docker build (or buildah, kaniko, etc). It cannot execute anything inside of the container as part of the build. It injects your build artifacts. That's it.
 
## Why?

Constructing containers inside of a k8s environment stinks. While there are a number of existing solutions, they all have substantial tradeoffs.

- DinD has considerable security implications, and depending on your environment may be a non-starter
- DinK is more secure, but it adds more moving pieces and has some performance issues (e.g. RWX PVCs)
- I have never gotten [buildah](https://github.com/containers/buildah) to work well in constrained environments (e.g. requiring something like `CAP_SETUID`). That being said, both it and kaniko _do more_. Much more. 
- [kaniko](https://github.com/GoogleContainerTools/kaniko) only supports being run in the published container. I've actually gotten the most mileage with kaniko in constrained environments (other than ko), but it often came with hacks or quirks to make it work how I wanted it to.

The complexity and tradeoffs are there to support the complex behavior that `Dockerfile`s can exhibit. However, in the 80%, I found I did not need that level of flexibility.

Enter `ko`. `ko` is a similar, simple, single binary. You call `ko build`. It doesn't require any privileges. It builds your app, packages it in a container, and ships it to your repo. Done.

Unfortunately, ko is only for go. If you're using go, and by some weird SEO quirk you ended up here instead. Stop. Go use [ko](https://ko.build).

## Other Options

Aside from kaniko and buildah, there are quite a few investigations in this space. I'm sure I'm missing some, but:

- [umoci](https://umo.ci/) and [crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md) are CLIs for interacting with OCI images. You can accomplish a lot with these and a shell script, but I wanted something simpler and easier to maintain.
- [stacker](https://github.com/project-stacker/stacker). I have not used it and can't vouch for it, but it seems to live in a middle ground between tko and something more like kaniko. In my case, it's added complexity did not seem worth it, but if you're looking in this space, it may be worth checking out.
- [apko](https://github.com/chainguard-dev/apko) + [melange](https://github.com/chainguard-dev/melange). The tooling story is pretty rough at the time of writing this, but I like the direction. If you're an enterprise, looking to do enterprise-y things, I would recommend checking them out.

## Examples

### Quarkus + Rootless Github Self Hosted Runners

```
- uses: graalvm/setup-graalvm@2f25c0caae5b220866f732832d5e3e29ff493338
  with:
    java-version: '17'
    distribution: 'mandrel'
    
- name: Build with Maven
  run: |
    ./mvnw -Dsha1=${{ github.sha }} package -Dnative --no-transfer-progress
    mkdir -p out
    mv target/*-runner out/app

- name: Publish
  run: tko "./out"
  env:
    TKO_BASE_IMAGE: debian:bookworm-slim@sha256:155280b00ee0133250f7159b567a07d7cd03b1645714c3a7458b2287b0ca83cb
    TKO_TARGET_REPO: ghcr.io/your-org/your-repo
    GITHUB_TOKEN: ${{ github.token }}
```
