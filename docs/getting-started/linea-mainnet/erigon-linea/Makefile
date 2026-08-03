IMAGE ?= linea-erigon
TAG ?= v3.3.0-linea-patched
PLATFORM ?= linux/amd64

.PHONY: build buildx-info

buildx-info:
	@docker buildx version
	@docker buildx ls

# Build locally and load into your Docker daemon.
# On Apple Silicon (arm64), run: make build PLATFORM=linux/arm64
build:
	docker buildx build \
		--platform $(PLATFORM) \
		--file Dockerfile \
		--tag $(IMAGE):$(TAG) \
		--progress=plain \
		--load \
		.

