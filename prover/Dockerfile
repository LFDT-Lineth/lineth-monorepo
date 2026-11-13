# syntax=docker/dockerfile:1
# kics-scan disable=d3499f6d-1651-41bb-a9a7-de925fea487b
# Unpinned Package Version in Apk Add: apk packages intentionally track the
# digest-pinned alpine base; version pinning would break the build on base
# refresh for negligible security gain on top of apk's --no-cache.
# Required to use the --build-context flag

###############################
#
# G O + R U S T   B U I L D E R
#
###############################
FROM golang:1.25.7-alpine@sha256:f6751d823c26342f9506c03797d2527668d095b0a15f1862cddb4d927a7a4ced AS go-builder

RUN apk add --no-cache rust cargo bash make

WORKDIR /usr/src/
COPY --from=prover go.mod .
COPY --from=prover go.sum .
RUN go mod download

COPY --from=prover . ./prover

WORKDIR /usr/src/prover/
RUN make bin/prover
RUN make bin/controller

#######################
#
# F I N A L   I M A G E
#
#######################
FROM alpine:3@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

RUN apk add --no-cache bash musl-dev libgcc
RUN mkdir -p /opt/linea/prover/lib/compressor /app/logs

COPY LICENSE-APACHE LICENSE-MIT /licenses/

# Copy the blob compressor dictionaries
COPY --from=go-builder /usr/src/prover/lib/compressor/compressor_dict.bin /opt/linea/prover/lib/compressor/
COPY --from=go-builder /usr/src/prover/lib/compressor/dict/25-04-21.bin /opt/linea/prover/lib/compressor/dict/

# Import the prover
COPY --from=go-builder /usr/src/prover/bin/prover /opt/linea/prover/prover
COPY --from=go-builder /usr/src/prover/bin/controller /opt/linea/prover/controller

# Initilize the dirs that will contain the configuration files and the setup
RUN mkdir -p /opt/linea/prover/config

# Run as a dedicated non-root user (uid 1000). The config dir is typically
# volume-mounted at runtime — make sure it is readable by uid 1000.
RUN adduser -D -u 1000 prover && \
    chown -R prover:prover /opt/linea/prover /app/logs

USER prover

# Useful ENV variables that you may want to pass at runtime
ENV GOMEMLIMIT=340GiB
ENV CONFIG_FILE="/opt/linea/prover/config/config.toml"

ENV PATH="${PATH}:/opt/linea/prover/"

CMD ["sh", "-c", "/opt/linea/prover/controller --config $CONFIG_FILE --local-id $WORKER_ID"]

# Build-time metadata as defined at http://label-schema.org
# Values are injected by scripts/docker/build-image.sh.
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="prover" \
	org.label-schema.description="Lineth prover" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/LFDT-Lineth/lineth-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0" \
	org.opencontainers.image.licenses="MIT OR Apache-2.0"
