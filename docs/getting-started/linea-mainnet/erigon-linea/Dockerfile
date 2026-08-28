# Erigon v3.3.0 patched for Linea
# Based on Keith C's findings: https://github.com/erigontech/erigon/issues/18160

# Base images pinned by digest for reproducible, tamper-resistant builds.
# Bump these digests when refreshing golang / debian patch updates.
FROM golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    && rm -rf /var/lib/apt/lists/*

# Clone Erigon at a pinned commit (v3.3.0 tag -> a0c55b4).
# Pinning by SHA guards against the lightweight v3.3.0 tag being moved upstream.
ARG ERIGON_REPO=https://github.com/erigontech/erigon.git
ARG ERIGON_COMMIT=a0c55b440aa6ac54b2583ff9a4232820eb7947ff
WORKDIR /build
RUN git init . && \
    git remote add origin ${ERIGON_REPO} && \
    git fetch --depth 1 origin ${ERIGON_COMMIT} && \
    git checkout ${ERIGON_COMMIT}

# Patch 1: Disable block downloader v2 (fixes corruption on EC2/slower machines).
# grep verifies the substitution applied; sed exits 0 even on a no-match.
RUN sed -i 's/const sentryMcDisableBlockDownload = true/const sentryMcDisableBlockDownload = false/' node/eth/backend.go && \
    grep -q 'const sentryMcDisableBlockDownload = false' node/eth/backend.go

# Patch 2: Return nil instead of error on empty code for EIP-7002 (Linea has empty withdrawal contract).
# Verify the original error string is gone (the replacement `return nil, nil` is too generic to grep for).
RUN sed -i 's/return nil, fmt.Errorf("\[EIP-7002\] Syscall failure: Empty Code at WithdrawalRequestAddress=%x", params.WithdrawalRequestAddress)/return nil, nil/' execution/protocol/rules/misc/eip7002.go && \
    ! grep -q 'Syscall failure: Empty Code at WithdrawalRequestAddress' execution/protocol/rules/misc/eip7002.go

# Patch 3: Return nil instead of error on empty code for EIP-7251 (Linea has empty consolidation contract).
RUN sed -i 's/return nil, fmt.Errorf("\[EIP-7251\] Syscall failure: Empty Code at ConsolidationRequestAddress=%x", params.ConsolidationRequestAddress)/return nil, nil/' execution/protocol/rules/misc/eip7251.go && \
    ! grep -q 'Syscall failure: Empty Code at ConsolidationRequestAddress' execution/protocol/rules/misc/eip7251.go

# Build Erigon without silkworm (simpler, no external library needed)
RUN make erigon BUILD_TAGS=nosilkworm

# Final image
FROM debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Run as a non-root user (matches the repo convention for runtime images).
# The compose erigon-init service runs as root and chowns /data to 1000:1000
# before erigon-node starts, so this user can write the chain data volume.
RUN useradd -m -u 1000 erigon
COPY --from=builder /build/build/bin/erigon /usr/local/bin/

EXPOSE 8545 8550 30303 30303/udp

USER erigon

ENTRYPOINT ["erigon"]
