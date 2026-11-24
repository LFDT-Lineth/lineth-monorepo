# syntax=docker/dockerfile:1
# kics-scan disable=965a08d7-ef86-4f14-8792-4a3b2098937e
# Apt Get Install Pin Version Not Defined: apt packages are intentionally unpinned.
# They track the digest-pinned debian base images; pinning would break the build
# on base refresh for negligible security gain on top of --no-install-recommends.
#
# One prover-ray image for all modes; the mode is set per pipeline in the mounted
# config (CONFIG_FILE), not on the command line.
# Build (from prover-ray/, so the positional context is prover-ray):
#   make docker-build
# which runs, in effect:
#   docker build -f Dockerfile --build-context licenses=<repo> \
#     -t consensys/lineth-prover-ray:dev .
#
# No guest is bundled: guest ELFs are mounted in the guests folder at run time.

########################################
# Go builder: the static prover binary
########################################
FROM docker.io/library/golang:1.27.1-bookworm@sha256:8d48e12ec56735e9358640898b9d9b9fcca110612ed8a5567438c0a1baa24e66 AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO off: the prover is pure Go (the R5 arithmetization compiles at startup from
# embedded source), so the binary is static and base-image independent.
# -ldflags "-s -w" strips the symbol table and DWARF for a smaller prod binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/prover ./cmd/prover

########################################
# Final runtime image: debian:bookworm-slim, with a shell so the CMD expands
# $CONFIG_FILE and $WORKER_ID.
########################################
FROM docker.io/library/debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS final
ARG VERSION
ARG VCS_REF
ARG BUILD_DATE
LABEL org.label-schema.version=$VERSION \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.build-date=$BUILD_DATE \
      org.opencontainers.image.licenses="MIT OR Apache-2.0"

# Go toolchain for gogen: zkc builds the circuit as Go code at run time. It goes
# before the prover binary so this large, stable layer is not rebuilt on every release.
COPY --from=go-builder /usr/local/go /usr/local/go
COPY --from=go-builder /out/prover /opt/lineth/prover-ray/prover
COPY --from=licenses LICENSE-APACHE LICENSE-MIT /licenses/

# GOTOOLCHAIN=local: no download. CGO off: no C compiler. GOCACHE in /tmp: any user can write it.
ENV PATH="/opt/lineth/prover-ray:/usr/local/go/bin:${PATH}" \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOCACHE=/tmp/go-build
WORKDIR /opt/lineth/prover-ray

# Run as a dedicated non-root user (uid 1000). Volumes mounted at runtime
# (e.g. /data, guests/) must be readable/writable by uid 1000 — chown them
# on the host (1000:1000) before docker run.
RUN useradd --uid 1000 --no-create-home --home-dir /opt/lineth/prover-ray prover-ray \
    && chown -R prover-ray:prover-ray /opt/lineth/prover-ray

# Numeric uid:gid — Kubernetes cannot verify runAsNonRoot against a named user.
USER 1000:1000

# Configuration is a TOML file. No default is baked in (fail loudly rather than
# run a wrong config): mount one and point CONFIG_FILE at it. The CMD runs the
# queue adapter, expanding CONFIG_FILE and WORKER_ID like the legacy prover.
# Samples: prover-ray/config/config-dev-mock.toml, config-dev-zkvm.toml,
# config-dev-partial.toml.
# dev-zkvm and dev-partial also need the guests folder (scripts/stage-guest.sh):
#   docker run -e CONFIG_FILE=/data/config.toml -e WORKER_ID=prover-ray-0 \
#              -v ./config-dev-zkvm.toml:/data/config.toml -v ./data:/data \
#              -v ./guests:/opt/lineth/prover-ray/guests:ro <image>
ENV CONFIG_FILE="/opt/lineth/prover-ray/config/config.toml"
CMD ["sh", "-c", "exec prover --config \"$CONFIG_FILE\" --local-id \"$WORKER_ID\""]
