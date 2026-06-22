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

docker-build-local-image:
	./gradlew :app:installDist
	docker build app --build-context=libs=./app/build/install/app/lib/ -t consensys/maru:local

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
