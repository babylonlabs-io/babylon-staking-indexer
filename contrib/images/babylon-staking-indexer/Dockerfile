FROM golang:1.24.3-alpine AS builder

ARG VERSION="HEAD"

RUN apk add --no-cache  \
    make \
    git \
    build-base \
    linux-headers \
    libc-dev \
    pkgconfig \
    alpine-sdk \
    libsodium-dev \
    libsodium-static \
    openssh

# Build
WORKDIR /go/src/github.com/babylonlabs-io/babylon-staking-indexer
# Cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonlabs-io/babylon-staking-indexer/

# Copy the rest of the files
COPY ./ /go/src/github.com/babylonlabs-io/babylon-staking-indexer/

# Cosmwasm - Download correct libwasmvm version
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
RUN WASMVM_VERSION=$(go list -m github.com/CosmWasm/wasmvm/v2 | cut -d ' ' -f 2) && \
    wget -q https://github.com/CosmWasm/wasmvm/releases/download/$WASMVM_VERSION/libwasmvm_muslc."$(uname -m)".a \
    -O /lib/libwasmvm_muslc."$(uname -m)".a && \
    # verify checksum
    wget -q https://github.com/CosmWasm/wasmvm/releases/download/"$WASMVM_VERSION"/checksums.txt -O /tmp/checksums.txt && \
    sha256sum /lib/libwasmvm_muslc."$(uname -m)".a | grep $(cat /tmp/checksums.txt | grep libwasmvm_muslc."$(uname -m)" | cut -d ' ' -f 1)

RUN LDFLAGS='-extldflags "-static" -v' \
    CGO_ENABLED=1 \
    CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
    LINK_STATICALLY=true \
    BUILD_TAGS="muslc" \
    make build

# Final minimal image with binary only
FROM alpine:3.16 as run

RUN addgroup --gid 1138 -S babylon-staking-indexer && adduser --uid 1138 -S babylon-staking-indexer -G babylon-staking-indexer
RUN apk add bash curl jq

# Label should match your github repo
LABEL org.opencontainers.image.source="https://github.com/babylonlabs-io/babylon-staking-indexer:${VERSION}"


# Copy over binaries from the build-env
COPY --from=builder /go/src/github.com/babylonlabs-io/babylon-staking-indexer/build/babylon-staking-indexer /bin/babylon-staking-indexer
COPY --from=builder /go/src/github.com/babylonlabs-io/babylon-staking-indexer/contrib/images/babylon-staking-indexer/entrypoint.sh /bin/entrypoint.sh

WORKDIR /home/babylon-staking-indexer
RUN chown -R babylon-staking-indexer /home/babylon-staking-indexer
USER babylon-staking-indexer

ENTRYPOINT ["/bin/entrypoint.sh"]
CMD []
STOPSIGNAL SIGTERM