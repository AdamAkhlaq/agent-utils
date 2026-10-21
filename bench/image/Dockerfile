# The benchmark images, one Dockerfile with three targets, built with the
# repository root as the context (go run ./bench does this):
#
#   base   the "shell" arm: Node, Claude Code, python3 (with PyYAML and
#          Pillow), jq, yq, openssl, ImageMagick, coreutils. Every checker
#          runs here too, so no checker can depend on agent-utils.
#   tool   base plus agent-utils, installed from a release tag so every
#          published number maps to a release, never to a working tree.
#   proxy  the CONNECT proxy from bench/proxy, a run's only egress.
#
# Everything is pinned: the Node and Go images by digest, Claude Code
# by npm version, yq by version and SHA-256, agent-utils by tag and the
# release's checksums. Changing a pin is a deliberate change to what the
# benchmark measures.

ARG NODE_IMAGE=node:22-bookworm-slim@sha256:de170b39439eb0e5a3444ad7d60b015614a381e10fecae2e30ec9e735122f38d

FROM ${NODE_IMAGE} AS base

ARG CLAUDE_CODE_VERSION=2.1.304
ARG YQ_VERSION=v4.54.1
ARG TARGETARCH

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates curl coreutils python3 python3-yaml python3-pil jq openssl imagemagick \
    && rm -rf /var/lib/apt/lists/*

RUN set -eu; \
    case "$TARGETARCH" in \
        amd64) sum=8e34fc298390875de416e6a4afcb8cabeceb25d9aa8506c1a2f9353cf702ea5f ;; \
        arm64) sum=189088da0c6429ec5178dfaab1a114805f6cab0b61b165ab236efedf1d57a71b ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_${TARGETARCH}"; \
    echo "$sum  /usr/local/bin/yq" | sha256sum -c -; \
    chmod 0755 /usr/local/bin/yq

RUN npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" && npm cache clean --force

# A non-root user: bypassPermissions refuses to run as root. /out and /work
# are created here so a named volume mounted at /out inherits bench's
# ownership and the user can write it.
RUN useradd --create-home --shell /bin/bash bench \
    && mkdir -p /out /work /task \
    && chown bench:bench /out /work

# Isolation switches that hold for every arm. The runner adds the per-arm
# flags and the credential at docker run time.
ENV CLAUDE_CONFIG_DIR=/home/bench/.claude-bench \
    CLAUDE_CODE_DISABLE_CLAUDE_MDS=1 \
    CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 \
    CLAUDE_CODE_SKIP_PROMPT_HISTORY=1 \
    DISABLE_AUTOUPDATER=1 \
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 \
    DISABLE_TELEMETRY=1 \
    DISABLE_ERROR_REPORTING=1

COPY --chmod=0755 bench/image/entrypoint.sh /usr/local/bin/entrypoint

USER bench
WORKDIR /work
ENTRYPOINT ["entrypoint"]

FROM base AS tool

ARG AGENT_UTILS_VERSION
ARG TARGETARCH

USER root
RUN set -eu; \
    test -n "$AGENT_UTILS_VERSION" || { echo "AGENT_UTILS_VERSION build arg is required" >&2; exit 1; }; \
    release="https://github.com/AdamAkhlaq/agent-utils/releases/download/${AGENT_UTILS_VERSION}"; \
    archive="agent-utils_linux_${TARGETARCH}.tar.gz"; \
    cd /tmp; \
    curl -fsSL -o "$archive" "${release}/${archive}"; \
    curl -fsSL -o checksums.txt "${release}/checksums.txt"; \
    grep " ${archive}\$" checksums.txt | sha256sum -c -; \
    tar -xzf "$archive" -C /usr/local/bin agent-utils; \
    chmod 0755 /usr/local/bin/agent-utils; \
    rm -f "$archive" checksums.txt; \
    agent-utils version
USER bench

FROM golang:1.26-bookworm@sha256:632dff7351799a5a9954a48dc06670ade72ee44e8c351cdc0e5a57af2fea21af AS proxy-build

WORKDIR /src
COPY go.mod go.sum ./
COPY bench/proxy ./bench/proxy
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /proxy ./bench/proxy

# The proxy is a static binary that never does TLS itself, so the image is
# the binary alone: no shell, no package manager, no certificates, and a
# fixed unprivileged UID.
FROM scratch AS proxy

COPY --from=proxy-build /proxy /proxy
USER 65534:65534
EXPOSE 3128
ENTRYPOINT ["/proxy"]
