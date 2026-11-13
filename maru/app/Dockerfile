# FINAL image
FROM eclipse-temurin:25-jre-noble@sha256:e3b6f7887a50f20d8f07abf279d8a81a5953fb398a9555bd9edd56b9bd15968c

WORKDIR /opt/consensys/maru

# Run as a dedicated non-root user (uid 1000, same convention as the besu image).
# Ubuntu noble ships an "ubuntu" user at uid 1000 — remove it first.
RUN userdel ubuntu 2>/dev/null || true && rm -rf /home/ubuntu && \
    adduser --uid 1000 --disabled-password --gecos "" --home /opt/consensys/maru maru && \
    chown -R maru:maru /opt/consensys/maru

COPY --from=libs --chown=maru:maru ./** ./
# The glob lets downstream builds whose licenses context has no license files skip them
# (scripts/docker/build-image.sh --no-license-check).
COPY --from=licenses LICENSE-* /licenses/

USER maru

ARG LINETH_APP_JAR=/opt/consensys/maru/maru.jar
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
