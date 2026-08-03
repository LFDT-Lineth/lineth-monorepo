# Erigon v3.3.0 patched for Linea
# Based on Keith C's findings: https://github.com/erigontech/erigon/issues/18160

FROM golang:1.24-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    && rm -rf /var/lib/apt/lists/*

# Clone Erigon v3.3.0
WORKDIR /build
RUN git clone --depth 1 --branch v3.3.0 https://github.com/erigontech/erigon.git .

# Patch 1: Disable block downloader v2 (fixes corruption on EC2/slower machines)
RUN sed -i 's/const sentryMcDisableBlockDownload = true/const sentryMcDisableBlockDownload = false/' node/eth/backend.go

# Patch 2: Return nil instead of error on empty code for EIP-7002 (Linea has empty withdrawal contract)
RUN sed -i 's/return nil, fmt.Errorf("\[EIP-7002\] Syscall failure: Empty Code at WithdrawalRequestAddress=%x", params.WithdrawalRequestAddress)/return nil, nil/' execution/protocol/rules/misc/eip7002.go

# Patch 3: Return nil instead of error on empty code for EIP-7251 (Linea has empty consolidation contract)
RUN sed -i 's/return nil, fmt.Errorf("\[EIP-7251\] Syscall failure: Empty Code at ConsolidationRequestAddress=%x", params.ConsolidationRequestAddress)/return nil, nil/' execution/protocol/rules/misc/eip7251.go

# Build Erigon without silkworm (simpler, no external library needed)
RUN make erigon BUILD_TAGS=nosilkworm

# Final image
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/build/bin/erigon /usr/local/bin/

EXPOSE 8545 8550 30303 30303/udp

ENTRYPOINT ["erigon"]
