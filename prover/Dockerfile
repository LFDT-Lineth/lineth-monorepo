# syntax=docker/dockerfile:1
# Required to use the --build-context flag

###############################
#
# G O + R U S T   B U I L D E R
#
###############################
FROM golang:1.22.7-alpine AS go-builder

RUN apk add --no-cache rust cargo bash make

WORKDIR /usr/src/
COPY --from=prover go.mod .
COPY --from=prover go.sum .
RUN go mod download

COPY --from=constraints . ./constraints
COPY --from=prover . ./prover

WORKDIR /usr/src/prover/
RUN make bin/prover
RUN make bin/controller

#######################
#
# F I N A L   I M A G E
#
#######################
FROM alpine:3

RUN apk add --no-cache bash musl-dev libgcc
RUN mkdir -p /opt/linea/prover/lib/compressor /data/work /app/logs

# Import the LICENSE
COPY ./LICENSE /opt/linea/prover/

# Copy the blob compressor dictionary
COPY --from=go-builder /usr/src/prover/lib/compressor/compressor_dict.bin /opt/linea/prover/lib/compressor/

# Import the prover
COPY --from=go-builder /usr/src/prover/bin/prover /opt/linea/prover/prover
COPY --from=go-builder /usr/src/prover/bin/controller /opt/linea/prover/controller

# Initilize the dirs that will contain the configuration files and the setup
RUN mkdir -p /opt/linea/prover/config

# Useful ENV variables that you may want to pass at runtime
ENV GOMEMLIMIT=340GiB
ENV CONFIG_FILE="/opt/linea/prover/config/config.toml"

ENV PATH="${PATH}:/opt/linea/prover/"

CMD ["sh", "-c", "/opt/linea/prover/controller --config $CONFIG_FILE --local-id $WORKER_ID"]
