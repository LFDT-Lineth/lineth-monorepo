# syntax=docker/dockerfile:1
#
# Builds one guest's release ELF into a scratch stage, with the pinned Zig.
# Use: make -C riscv-guests docker-artifacts GUEST=l2-execution

FROM docker.io/library/debian:bookworm AS builder
ARG ZIG_VERSION
ARG ZIG_SHA256
ARG ZIG_URL=https://ziglang.org/download/${ZIG_VERSION}/zig-x86_64-linux-${ZIG_VERSION}.tar.xz
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && apt-get install -y --no-install-recommends \
        ca-certificates curl xz-utils git make build-essential python3 \
    && rm -rf /var/lib/apt/lists/*
RUN test -n "${ZIG_VERSION}" && test -n "${ZIG_SHA256}" \
    && curl -fsSL "${ZIG_URL}" -o /tmp/zig.tar.xz \
    && echo "${ZIG_SHA256}  /tmp/zig.tar.xz" | sha256sum --check --status \
    && tar -xJf /tmp/zig.tar.xz -C /opt \
    && ln -s "/opt/zig-x86_64-linux-${ZIG_VERSION}/zig" /usr/local/bin/zig \
    && rm /tmp/zig.tar.xz
COPY Makefile /riscv-guests/Makefile
RUN make -C /riscv-guests install-constantine-deps \
    && rm -rf /var/lib/apt/lists/*
ENV PATH="/root/.local/bin:${PATH}"
COPY . /riscv-guests
ARG GUEST
ARG RELEASE_VERSION
RUN test -n "${GUEST}" && test -n "${RELEASE_VERSION}" \
    && make -C /riscv-guests release-asset GUEST="${GUEST}" \
        RELEASE_VERSION="${RELEASE_VERSION}" RELEASE_DIR=/out

FROM scratch AS artifacts
COPY --from=builder /out/ /
