# BUILDER image
FROM eclipse-temurin:25-jre-noble

WORKDIR /opt/consensys/maru

COPY --from=libs ./** ./
# The glob lets downstream builds whose licenses context has no license files skip them
# (scripts/docker/build-image.sh --no-license-check).
COPY --from=licenses LICENSE-* /licenses/

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
