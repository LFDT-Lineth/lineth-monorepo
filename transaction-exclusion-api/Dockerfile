FROM eclipse-temurin:25-jre-noble@sha256:e3b6f7887a50f20d8f07abf279d8a81a5953fb398a9555bd9edd56b9bd15968c
WORKDIR /opt/consensys/linea/transaction-exclusion-api/

# Run as a dedicated non-root user (uid 1000, same convention as the besu image).
# Ubuntu noble ships an "ubuntu" user at uid 1000 — remove it first.
RUN userdel ubuntu 2>/dev/null || true && rm -rf /home/ubuntu && \
    adduser --uid 1000 --disabled-password --gecos "" --home /opt/consensys/linea/transaction-exclusion-api txexclusion && \
    mkdir -p logs && \
    chown -R txexclusion:txexclusion /opt/consensys/linea/transaction-exclusion-api

COPY --from=libs --chown=txexclusion:txexclusion ./** libs/
COPY LICENSE-APACHE LICENSE-MIT /licenses/

USER txexclusion

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="transaction-exclusion-api" \
	org.label-schema.description="Lineth Transaction Exclusion API" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/LFDT-Lineth/lineth-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0" \
	org.opencontainers.image.licenses="MIT OR Apache-2.0"
