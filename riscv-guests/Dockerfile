# syntax=docker/dockerfile:1
# kics-scan disable=965a08d7-ef86-4f14-8792-4a3b2098937e
# Apt Get Install Pin Version Not Defined: apt packages are intentionally unpinned.
# They track the digest-pinned debian base image; pinning would break the build
# on base refresh for negligible security gain on top of --no-install-recommends.
#
# Builds one guest's release ELF into a scratch stage, with the pinned Zig.
# Use: make -C riscv-guests docker-artifacts GUEST=l2-execution

FROM docker.io/library/debian:bookworm@sha256:268fb24c39bca92d326dd83ea3abebe4a3f7751c804ffef23bb6d857dd343ff8 AS builder
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

# kics-scan ignore-line
# Missing User Instruction: this stage is never run as a container — it is an
# artifacts-only export (`--target artifacts --output type=local`), and scratch
# has no users to switch to. KICS query fd54f200-402c-4333-a5a4-36ef6709af2f.
FROM scratch AS artifacts
COPY --from=builder /out/ /
