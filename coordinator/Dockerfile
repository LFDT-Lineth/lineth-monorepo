# BUILDER image
FROM eclipse-temurin:25-jdk-noble AS builder

RUN mkdir -p /libs
COPY --from=libs ./** /libs
WORKDIR /libs

RUN mkdir -p unpacked-blob-compressor unpacked-blob-shnarf-calculator

WORKDIR /libs/unpacked-blob-compressor
RUN jar -xf $(set -- ../blob-compressor*.jar; echo $1)

WORKDIR /libs/unpacked-blob-shnarf-calculator
RUN jar -xf $(set -- ../blob-shnarf-calculator*.jar; echo $1)

WORKDIR /libs
RUN bcJar=$(set -- blob-compressor*.jar; echo $1) && \
    bscJar=$(set -- blob-shnarf-calculator*.jar; echo $1) && \
    rm -rf unpacked-blob-compressor/darwin-** unpacked-blob-shnarf-calculator/darwin-** && \
    case $(uname -m) in \
    x86_64) \
      rm -rf unpacked-blob-compressor/linux-aarch64/; \
      rm -rf unpacked-blob-shnarf-calculator/linux-aarch64/; \
    ;; \
    aarch64) \
      rm -rf unpacked-blob-compressor/linux-x86-64/; \
      rm -rf unpacked-blob-shnarf-calculator/linux-x86-64/; \
    ;; \
    esac && \
    jar -cf $bcJar -C unpacked-blob-compressor . && \
    jar -cf $bscJar -C unpacked-blob-shnarf-calculator .
RUN rm -rf unpacked-blob-compressor unpacked-blob-shnarf-calculator

# FINAL image
FROM eclipse-temurin:25-jre-noble

RUN groupmod -n coordinator ubuntu && \
    usermod -l coordinator -d /opt/consensys/linea/coordinator -s /usr/sbin/nologin -G "" ubuntu

WORKDIR /opt/consensys/linea/coordinator

RUN mkdir -p logs && \
    chown -R coordinator:coordinator /opt/consensys/linea/coordinator

COPY --from=builder --chown=coordinator:coordinator /libs/** libs/

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="coordinator" \
	org.label-schema.description="Coordinator for Linea" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/LFDT-Lineth/lineth-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0"

ENV COORDINATOR_OBSERVABILITY_PORT=9545 \
    COORDINATOR_JSON_RPC_PORT=9546

EXPOSE $COORDINATOR_OBSERVABILITY_PORT $COORDINATOR_JSON_RPC_PORT

USER coordinator

# Follows `config__override__api__observability_port` (the coordinator's own config-override
# env var convention, see Hoplite EnvironmentVariableOverridePropertySource) when set, since that
# is what actually changes the running port; otherwise falls back to COORDINATOR_OBSERVABILITY_PORT.
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD bash -c 'port="${config__override__api__observability_port:-$COORDINATOR_OBSERVABILITY_PORT}"; \
        exec 3<>/dev/tcp/localhost/$port && \
        echo -e "GET /health HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3 && \
        head -1 <&3 | grep -q "200"' || exit 1
