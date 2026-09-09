# Build the manager binary
# --platform=$BUILDPLATFORM pins this stage to the build machine's own arch regardless of TARGETARCH/TARGETOS
# below, so a multi-arch buildx build cross-compiles the Go binary natively instead of running the whole builder
# stage under QEMU emulation for each non-native target - the final distroless stage below has no RUN steps, so
# it needs no emulation either way, making the entire multi-platform build emulation-free end to end.
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY controllers/ controllers/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
# -trimpath and -ldflags="-s -w" drop build-machine file paths and debug symbols (smaller binary, nothing
# about where it was built leaked into it); -X main.version stamps in what was actually built, since the
# distroless final image below has no shell to run `manager --version` against otherwise.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o manager cmd/main.go

# distroless/static:nonroot (kubebuilder's own default) ships no shell and no package manager - nothing for
# a CVE scanner to flag beyond the Go binary itself - while still including ca-certificates and a working
# nonroot user. That user is already UID/GID 65532, the same one this image ran as before under a manually
# created Alpine user, so nothing depending on that UID elsewhere (e.g. pod securityContexts) needs to change.
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
