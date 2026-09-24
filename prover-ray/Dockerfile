# syntax=docker/dockerfile:1
#
# One prover-ray image for all modes; the mode is chosen at runtime via --mode.
# Build (from prover-ray/, so the positional context is prover-ray):
#   make docker-build
# which runs, in effect:
#   docker build -f Dockerfile \
#     --build-context riscv-guests=<repo>/riscv-guests \
#     -t consensys/linea-prover-ray:dev .
#
# dev-mock needs only the Go binary. dev-zkvm additionally uses the bundled
# native runner (glibc, links libmcl.so) and the guest ELF, both built here.

########################################
# Go builder: the static prover binary
########################################
FROM docker.io/library/golang:1.25.7-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO off: the prover is pure Go (the R5 arithmetization compiles at startup from
# embedded source), so the binary is static and base-image independent.
RUN CGO_ENABLED=0 go build -trimpath -o /out/prover ./cmd/prover

########################################
# Zig builder: the native runner + guest ELF
########################################
FROM docker.io/library/ubuntu:24.04 AS zig-builder
ARG ZIG_VERSION=0.16.0
# Override if 0.16.0 is not a tagged release (e.g. a nightly under /builds/).
ARG ZIG_URL=https://ziglang.org/download/${ZIG_VERSION}/zig-x86_64-linux-${ZIG_VERSION}.tar.xz
ARG BLST_SHA=e7f90de551e8df682f3cc99067d204d8b90d27ad
ARG MCL_SHA=0499298adcfad3bbcebf77f17700ebbe97166060
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && apt-get install -y --no-install-recommends \
        ca-certificates curl xz-utils git make build-essential python3 \
        libsecp256k1-dev libssl-dev \
    && rm -rf /var/lib/apt/lists/*
# Zig toolchain (pinned; see riscv-guests/l2-execution/build.zig.zon).
RUN curl -fsSL "${ZIG_URL}" -o /tmp/zig.tar.xz \
    && tar -xJf /tmp/zig.tar.xz -C /opt \
    && ln -s "/opt/zig-x86_64-linux-${ZIG_VERSION}/zig" /usr/local/bin/zig \
    && rm /tmp/zig.tar.xz
# Native crypto deps (pinned SHAs, mirroring riscv-guests install-native-crypto-deps).
RUN git clone https://github.com/supranational/blst.git /tmp/blst \
    && git -C /tmp/blst checkout --detach ${BLST_SHA} \
    && (cd /tmp/blst && ./build.sh) \
    && cp /tmp/blst/libblst.a /usr/local/lib/ \
    && cp /tmp/blst/bindings/blst.h /tmp/blst/bindings/blst_aux.h /usr/local/include/ \
    && rm -rf /tmp/blst
RUN git clone https://github.com/herumi/mcl.git /tmp/mcl \
    && git -C /tmp/mcl checkout --detach ${MCL_SHA} \
    && make -C /tmp/mcl -j"$(nproc)" \
    && cp /tmp/mcl/lib/libmcl.so /usr/local/lib/ \
    && cp -r /tmp/mcl/include/mcl /usr/local/include/ \
    && ldconfig \
    && rm -rf /tmp/mcl
# Build the statically-linked rv64im guest ELF + the native runner. Path deps
# (build_common, guest-common, lineth-accelerators) are siblings in riscv-guests;
# the rest are fetched by zig from the network.
COPY --from=riscv-guests . /riscv-guests
RUN make -C /riscv-guests/l2-execution compile

########################################
# Final runtime image (glibc, for the native runner). Ubuntu 24.04 for now,
# because it has a shell for first-build debugging.
#
# PLANNED SLIM STEP (once the build is green): switch this final stage to
# gcr.io/distroless/cc-debian12 (~55MB total, no shell; logs still go to stdout).
# When doing so, ALSO switch the zig-builder above to debian:bookworm, so the
# native runner is built on the same glibc it runs on (debian12 = glibc 2.36);
# building on ubuntu's newer glibc and running on debian's older one can break.
########################################
FROM docker.io/library/ubuntu:24.04 AS final
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && apt-get install -y --no-install-recommends \
        ca-certificates libstdc++6 libgcc-s1 \
    && rm -rf /var/lib/apt/lists/*
# The native runner links these; copy them from the builder (same ubuntu 24.04,
# so same glibc/ABI). libmcl.so is source-built; libsecp256k1/libcrypto come from
# apt — glob the SONAME so we don't hard-code a version that may differ.
COPY --from=zig-builder /usr/local/lib/libmcl.so /usr/local/lib/
COPY --from=zig-builder /usr/lib/x86_64-linux-gnu/libsecp256k1.so* /usr/lib/x86_64-linux-gnu/
COPY --from=zig-builder /usr/lib/x86_64-linux-gnu/libcrypto.so* /usr/lib/x86_64-linux-gnu/
RUN ldconfig

# Prover binary (static), native runner, and guest ELF.
COPY --from=go-builder  /out/prover                                             /opt/linea/prover-ray/prover
COPY --from=zig-builder /riscv-guests/l2-execution/zig-out/bin/l2-execution-runner /opt/linea/prover-ray/l2-execution-runner
COPY --from=zig-builder /riscv-guests/l2-execution/zig-out/bin/evm_execution_guest /opt/linea/prover-ray/evm_execution_guest

RUN mkdir -p /opt/linea/prover-ray/requests
ENV PATH="${PATH}:/opt/linea/prover-ray"
WORKDIR /opt/linea/prover-ray

# Mode is a runtime choice. Default: dev-mock (needs nothing extra). For dev-zkvm:
#   docker run ... <image> --requests-dir /data --mode dev-zkvm \
#     --native-runner-bin /opt/linea/prover-ray/l2-execution-runner \
#     --guest-elf /opt/linea/prover-ray/evm_execution_guest
ENTRYPOINT ["/opt/linea/prover-ray/prover"]
CMD ["--requests-dir", "/opt/linea/prover-ray/requests", "--mode", "dev-mock"]
