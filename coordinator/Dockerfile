# BUILDER image
FROM eclipse-temurin:25-jdk-noble AS builder

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
FROM eclipse-temurin:25-jre-noble

WORKDIR /opt/consensys/linea/coordinator

RUN mkdir -p logs /tmp/prover/request /tmp/prover/response

COPY --from=builder /libs/** libs/
# The glob lets downstream builds whose context has no license files skip them
# (scripts/docker/build-image.sh --no-license-check).
COPY LICENSE-* /licenses/

ARG LINETH_APP_JAR=/opt/consensys/linea/coordinator/libs/coordinator.jar
ENV LINETH_APP_JAR=$LINETH_APP_JAR

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
ARG IMAGE_LICENSES="MIT OR Apache-2.0"
ARG IMAGE_NAME="coordinator"
ARG IMAGE_DESCRIPTION="Coordinator for Lineth"
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
