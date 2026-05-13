docker-start:
	$(MAKE) docker-pull
	docker compose -f compose.yaml up -d

docker-stop:
	docker compose -f compose.yaml down

docker-clean:
	$(MAKE) docker-stop
	docker compose -f compose.yaml rm -v

docker-clean-start:
	$(MAKE) docker-clean
	$(MAKE) docker-start

docker-pull:
	docker compose -f compose.yaml pull
