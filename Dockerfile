# syntax=docker/dockerfile:1
# Container image for ipamthing. Two stages: compile the static binary with
# the Go toolchain go.mod names, then copy it into a small Alpine runtime.
# Alpine (not distroless) because the scanner shells out to `ip neigh show`
# from iproute2 for IPv6 neighbor lookups; without it the fallback is
# /proc/net/arp, which only sees IPv4 MACs.
#
# The build stage runs on the BUILD platform and cross-compiles with GOARCH
# (pure Go: no cgo), so a multi-arch build needs no QEMU emulation.
#
# go.mod decides the compiler, not the builder image: GOTOOLCHAIN is set to
# the version go.mod names, so the image is built with exactly the Go the
# release binaries are built with (setup-go reads the same file) and the
# govulncheck job's stdlib findings are fixed in one place. The golang tag
# below is only the bootstrap; when it is newer than go.mod the matching
# toolchain is downloaded once per build (cached by buildx).
#
# Base images are pinned by digest like the workflow actions are pinned by
# SHA; Dependabot (docker ecosystem) refreshes them. Keep the tag in the
# comment so the pin stays readable.
#
#   docker build --build-arg VERSION=$(git describe --tags) -t ipamthing .
#   docker run -d --network host --cap-add NET_RAW \
#     -v ipamthing-data:/var/lib/ipamthing ipamthing

# --- build stage ------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
WORKDIR /src

# Module download is its own layer so source edits do not refetch it.
COPY go.mod go.sum ./
# The `toolchain` line if present, else the `go` line: the same rule the go
# command applies. Written to a file so the build step reads the same value.
RUN tc=$(sed -n 's/^toolchain //p' go.mod); [ -n "$tc" ] || tc="go$(sed -n 's/^go //p' go.mod)"; \
    echo "$tc" > /toolchain && GOTOOLCHAIN="$tc" go mod download

COPY . .
ARG TARGETOS TARGETARCH VERSION=dev
# Same flags as `make build`. THIRD_PARTY_LICENSES.md is taken as committed;
# the release workflow verifies it against the module graph before any image
# is pushed (make notices-check).
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOTOOLCHAIN="$(cat /toolchain)" \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ipamthing . \
 && go version -m /out/ipamthing | head -1

# --- runtime stage ----------------------------------------------------------
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache iproute2 \
 && addgroup -S ipamthing \
 && adduser -S -G ipamthing -H -h /var/lib/ipamthing ipamthing \
 && mkdir -p /var/lib/ipamthing \
 && chown ipamthing:ipamthing /var/lib/ipamthing

COPY --from=build /out/ipamthing /usr/local/bin/ipamthing

USER ipamthing
EXPOSE 8080
VOLUME ["/var/lib/ipamthing"]

ENTRYPOINT ["/usr/local/bin/ipamthing"]
CMD ["-addr=:8080", "-db=/var/lib/ipamthing/ipamthing.db"]
