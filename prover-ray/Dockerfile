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
#   docker build -f Dockerfile \
#     --build-context riscv-guests=<repo>/riscv-guests \
#     --build-context licenses=<repo> \
#     -t consensys/lineth-prover-ray:dev .
#
# dev-mock needs only the Go binary. dev-zkvm additionally uses the bundled
# native runner (glibc) and the guest ELF, both built here from
# the same riscv-guests source so they always agree.

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
# Zig builder: the native runner + guest ELF
########################################
# debian:bookworm = glibc 2.36, matching the debian:bookworm-slim final stage,
# so the native runner is built on the same glibc it runs on.
FROM docker.io/library/debian:bookworm@sha256:268fb24c39bca92d326dd83ea3abebe4a3f7751c804ffef23bb6d857dd343ff8 AS zig-builder
ARG ZIG_VERSION=0.16.0
# Override if 0.16.0 is not a tagged release (e.g. a nightly under /builds/).
# When overriding ZIG_URL, override ZIG_SHA256 to match — the download is
# verified against it (checksums: https://ziglang.org/download/index.json).
ARG ZIG_URL=https://ziglang.org/download/${ZIG_VERSION}/zig-x86_64-linux-${ZIG_VERSION}.tar.xz
ARG ZIG_SHA256=70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && apt-get install -y --no-install-recommends \
        ca-certificates curl xz-utils git make build-essential python3 \
    && rm -rf /var/lib/apt/lists/*
# Zig toolchain (pinned version + verified checksum; see riscv-guests/l2_execution/build.zig.zon).
RUN curl -fsSL "${ZIG_URL}" -o /tmp/zig.tar.xz \
    && echo "${ZIG_SHA256}  /tmp/zig.tar.xz" | sha256sum -c - \
    && tar -xJf /tmp/zig.tar.xz -C /opt \
    && ln -s "/opt/zig-x86_64-linux-${ZIG_VERSION}/zig" /usr/local/bin/zig \
    && rm /tmp/zig.tar.xz
# Build the statically-linked rv64im guest ELF + the native runner. Path deps
# (build_common, guest_common, lineth_accelerators) are siblings in riscv-guests;
# the rest are fetched by zig from the network.
COPY --from=riscv-guests Makefile /riscv-guests/Makefile
RUN make -C /riscv-guests install-constantine-deps \
    && rm -rf /var/lib/apt/lists/*
ENV PATH="/root/.local/bin:${PATH}"
COPY --from=riscv-guests . /riscv-guests
# Strip the native runner's debug_info (the guest ELF is already stripped); the
# ReleaseSafe build keeps its runtime checks, we just drop symbols for prod size.
RUN make -C /riscv-guests/l2_execution fetch-host \
    && make -C /riscv-guests/l2_execution compile \
    && cd /riscv-guests/l2_execution \
    && zig build install-runner -Dhost-tools=true --summary all \
    && strip /riscv-guests/l2_execution/zig-out/bin/l2_execution_runner

########################################
# Final runtime image: debian:bookworm-slim (glibc 2.36, same release as the
# builders, with a shell so the CMD expands $CONFIG_FILE and $WORKER_ID like the
# legacy prover). The native runner's crypto (Constantine) is linked statically,
# so it only needs glibc from the base image.
########################################
FROM docker.io/library/debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS final
ARG VERSION
ARG VCS_REF
ARG BUILD_DATE
LABEL org.label-schema.version=$VERSION \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.build-date=$BUILD_DATE \
      org.opencontainers.image.licenses="MIT OR Apache-2.0"

# Prover binary (static), native runner, and guest ELF.
COPY --from=go-builder  /out/prover                                             /opt/linea/prover-ray/prover
COPY --from=zig-builder /riscv-guests/l2_execution/zig-out/bin/l2_execution_runner /opt/linea/prover-ray/l2_execution_runner
COPY --from=zig-builder /riscv-guests/l2_execution/zig-out/bin/evm_execution_guest /opt/linea/prover-ray/evm_execution_guest
COPY --from=licenses LICENSE-APACHE LICENSE-MIT /licenses/

ENV PATH="/opt/linea/prover-ray:${PATH}"
WORKDIR /opt/linea/prover-ray

# Run as a dedicated non-root user (uid 1000). Volumes mounted at runtime
# (e.g. /data, guests/) must be readable/writable by uid 1000 — chown them
# on the host (1000:1000) before docker run.
RUN useradd --uid 1000 --no-create-home --home-dir /opt/linea/prover-ray prover-ray \
    && chown -R prover-ray:prover-ray /opt/linea/prover-ray

USER prover-ray

# Configuration is a TOML file. No default is baked in (fail loudly rather than
# run a wrong config): mount one and point CONFIG_FILE at it. The CMD runs the
# queue adapter, expanding CONFIG_FILE and WORKER_ID like the legacy prover.
# Samples: prover-ray/config/config-dev-mock.toml, config-dev-zkvm.toml.
# dev-zkvm also needs the guests folder, staged with scripts/stage-guest.sh:
#   docker run -e CONFIG_FILE=/data/config.toml -e WORKER_ID=prover-ray-0 \
#              -v ./config-dev-zkvm.toml:/data/config.toml -v ./data:/data \
#              -v ./guests:/opt/linea/prover-ray/guests:ro <image>
ENV CONFIG_FILE="/opt/linea/prover-ray/config/config.toml"
CMD ["sh", "-c", "exec prover --config \"$CONFIG_FILE\" --local-id \"$WORKER_ID\""]
