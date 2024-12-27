FROM golang:1.23.4-alpine AS builder

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