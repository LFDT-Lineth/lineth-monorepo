# BUILDER image
FROM eclipse-temurin:25-jre-noble

WORKDIR /opt/consensys/maru

COPY --from=libs ./** ./

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="maru" \
	org.label-schema.description="Consensus Client for Linea" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/ConsenSys/maru" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0"
