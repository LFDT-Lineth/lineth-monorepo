# BUILDER image
FROM eclipse-temurin:25-jre-noble@sha256:f3dd0838e69b881cc382237d5901e74ae3705b1056ad964b2e0e523b174fef99

WORKDIR /opt/lineth/maru

# Run as a dedicated non-root user (uid 1000, same convention as the besu image).
# Ubuntu noble ships an "ubuntu" user at uid 1000 — remove it first.
# /tmp/maru-db is maru's persistence data-path (docker/config/maru/sequencer.config.toml);
# pre-create it maru-owned so bind-mounting the private key file into it does not
# leave a root-owned directory that uid 1000 cannot write the database into.
RUN userdel ubuntu 2>/dev/null || true && rm -rf /home/ubuntu && \
    adduser --uid 1000 --disabled-password --gecos "" --home /opt/lineth/maru maru && \
    mkdir -p /tmp/maru-db && \
    chown -R maru:maru /opt/lineth/maru /tmp/maru-db

# Keep the libs tree so downstream builds can ship nested layouts.
COPY --from=libs --chown=maru:maru  . ./
# The glob lets downstream builds whose licenses context has no license files skip them
# (scripts/docker/build-image.sh --no-license-check).
COPY --from=licenses LICENSE-* /licenses/

# Numeric uid:gid — Kubernetes cannot verify runAsNonRoot against a named user.
USER 1000:1000

ARG LINETH_APP_JAR=/opt/lineth/maru/maru.jar
ENV LINETH_APP_JAR=$LINETH_APP_JAR

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
ARG IMAGE_LICENSES="MIT OR Apache-2.0"
ARG IMAGE_NAME="maru"
ARG IMAGE_DESCRIPTION="Consensus Client for Linea"
ARG IMAGE_VCS_URL="https://github.com/LFDT-Lineth/lineth-monorepo"
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="$IMAGE_NAME" \
	org.label-schema.description="$IMAGE_DESCRIPTION" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="$IMAGE_VCS_URL" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0" \
	org.opencontainers.image.licenses="$IMAGE_LICENSES"
