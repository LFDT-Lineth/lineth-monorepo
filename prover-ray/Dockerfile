# syntax=docker/dockerfile:1
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
FROM docker.io/library/golang:1.27.1-bookworm AS go-builder
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
FROM docker.io/library/debian:bookworm-slim AS final
ARG VERSION
ARG VCS_REF
ARG BUILD_DATE
LABEL org.label-schema.version=$VERSION \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.build-date=$BUILD_DATE \
      org.opencontainers.image.licenses="MIT OR Apache-2.0"

COPY --from=go-builder /out/prover /opt/linea/prover-ray/prover
COPY --from=licenses LICENSE-APACHE LICENSE-MIT /licenses/
# The Go toolchain for gogen (top-level setting, on by default): ZkC Execute
# builds the circuit as Go code at run time. GOTOOLCHAIN=local never downloads
# another toolchain, and CGO off needs no C compiler.
COPY --from=go-builder /usr/local/go /usr/local/go

ENV PATH="/opt/linea/prover-ray:/usr/local/go/bin:${PATH}" \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0
WORKDIR /opt/linea/prover-ray

# Configuration is a TOML file. No default is baked in (fail loudly rather than
# run a wrong config): mount one and point CONFIG_FILE at it. The CMD runs the
# queue adapter, expanding CONFIG_FILE and WORKER_ID like the legacy prover.
# Samples: prover-ray/config/config-dev-mock.toml, config-dev-zkvm.toml,
# config-dev-partial.toml.
# dev-zkvm and dev-partial also need the guests folder (scripts/stage-guest.sh):
#   docker run -e CONFIG_FILE=/data/config.toml -e WORKER_ID=prover-ray-0 \
#              -v ./config-dev-zkvm.toml:/data/config.toml -v ./data:/data \
#              -v ./guests:/opt/linea/prover-ray/guests:ro <image>
ENV CONFIG_FILE="/opt/linea/prover-ray/config/config.toml"
CMD ["sh", "-c", "exec prover --config \"$CONFIG_FILE\" --local-id \"$WORKER_ID\""]
