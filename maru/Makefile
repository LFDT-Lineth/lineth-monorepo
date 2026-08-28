.PHONY: build

build:
	./gradlew build

spotless-happy:
	./gradlew spotlessApply

pre-commit:
	$(MAKE) spotless-happy
	$(MAKE) build

run-e2e-test:
	./gradlew e2e:acceptanceTest

clean:
	./gradlew clean

# Delegates to the monorepo-wide recipe in scripts/docker/images.mk, which mirrors
# .github/workflows/maru-build-and-publish.yml. Produces consensys/maru:local.
# Overrides propagate, e.g. `make docker-build-local-image PLATFORMS=linux/arm64`.
docker-build-local-image:
	$(MAKE) -C .. docker-build-maru

docker-run-stack:
	CREATE_EMPTY_BLOCKS=true $(if $(MARU_TAG),MARU_TAG=$(MARU_TAG)) docker compose -f docker/compose.yaml -f docker/compose.dev.yaml up -d

docker-run-stack-partial:
	CREATE_EMPTY_BLOCKS=true $(if $(MARU_TAG),MARU_TAG=$(MARU_TAG)) docker compose -f docker/compose.yaml -f docker/compose.dev.yaml up -d maru

docker-clean-environment:
	docker compose -f docker/compose.yaml -f docker/compose.dev.yaml down || true
	docker volume rm maru-local-dev maru-logs || true # ignore failure if volumes do not exist already
	docker system prune -f || true
	rm docker/initialization/*.json || true # ignore failure if files do not exist already

docker-pull:
	cd docker/; \
	make docker-pull
