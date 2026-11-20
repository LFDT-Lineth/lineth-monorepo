FROM eclipse-temurin:25-jre-noble
WORKDIR /opt/consensys/linea/transaction-exclusion-api/

RUN mkdir -p logs

COPY --from=libs ./** libs/
COPY LICENSE-APACHE LICENSE-MIT /licenses/

# Remove once the base image ships Temurin 25.0.5.1+2 or later.
ENV JAVA_TOOL_OPTIONS="-XX:-UseBASE64Intrinsics"

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
