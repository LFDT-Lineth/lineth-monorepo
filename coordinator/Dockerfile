# BUILDER image
FROM eclipse-temurin:21-jdk-noble AS builder

RUN mkdir -p /libs
COPY --from=libs ./** /libs
WORKDIR /libs

RUN mkdir -p unpacked-blob-compressor unpacked-blob-shnarf-calculator
RUN bcJar=$(set -- blob-compressor*.jar; echo $1) && \
    bscJar=$(set -- blob-shnarf-calculator*.jar; echo $1) && \
    cd unpacked-blob-compressor/ && jar -xf ../$bcJar && cd .. && \
    cd unpacked-blob-shnarf-calculator/ && jar -xf ../$bscJar && cd .. && \
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
FROM eclipse-temurin:21-jre-noble

WORKDIR /opt/consensys/linea/coordinator

RUN mkdir -p logs /tmp/prover/request /tmp/prover/response

COPY --from=builder /libs/** libs/

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="coordinator" \
	org.label-schema.description="Coordinator for Linea" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/ConsenSys/linea-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0"
