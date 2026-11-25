# BUILDER image
FROM eclipse-temurin:25-jdk-noble@sha256:4ddc1a8bdaf283421df5487411aff10b797ccf1e06a339041005114c8f489b5c AS builder

RUN mkdir -p /libs
# Keep the libs tree so downstream builds can ship nested layouts.
COPY --from=libs . /libs
WORKDIR /libs

RUN mkdir -p unpacked-blob-compressor unpacked-blob-shnarf-calculator
RUN bcJar=$(find /libs -name 'blob-compressor*.jar') && \
    bscJar=$(find /libs -name 'blob-shnarf-calculator*.jar') && \
    cd unpacked-blob-compressor/ && jar -xf $bcJar && cd .. && \
    cd unpacked-blob-shnarf-calculator/ && jar -xf $bscJar && cd .. && \
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
FROM eclipse-temurin:25-jre-noble@sha256:f3dd0838e69b881cc382237d5901e74ae3705b1056ad964b2e0e523b174fef99

WORKDIR /opt/lineth/coordinator

# Run as a dedicated non-root user (uid 1000, same convention as the besu image).
# Ubuntu noble ships an "ubuntu" user at uid 1000 — remove it first.
RUN userdel ubuntu 2>/dev/null || true && rm -rf /home/ubuntu && \
    adduser --uid 1000 --disabled-password --gecos "" --home /opt/lineth/coordinator coordinator && \
    chown -R coordinator:coordinator /opt/lineth/coordinator

COPY --from=builder --chown=coordinator:coordinator /libs/ libs/
# The glob lets downstream builds whose context has no license files skip them
# (scripts/docker/build-image.sh --no-license-check).
COPY LICENSE-* /licenses/

# Numeric uid:gid — Kubernetes cannot verify runAsNonRoot against a named user.
USER 1000:1000

ARG LINETH_APP_JAR=/opt/lineth/coordinator/libs/coordinator.jar
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
