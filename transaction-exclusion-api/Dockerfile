FROM eclipse-temurin:21-jre-alpine
WORKDIR /opt/consensys/linea/transaction-exclusion-api/

RUN mkdir -p logs

COPY --from=libs ./** libs/

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="transaction-exclusion-api" \
	org.label-schema.description="Linea Transaction Exclusion API" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/ConsenSys/linea-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0"
