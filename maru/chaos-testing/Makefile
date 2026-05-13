include chaos-mesh.mk
include helm.mk
include k3s.mk
include benchmark.mk

.PHONY: chaos-full-reload \
	chaos-mesh-install-with-curl \
	chaos-mesh-install \
	chaos-mesh-uninstall \
	chaos-mesh-run-network-experiment \
	chaos-mesh-reload \
	helm-clean-releases \
	helm-redeploy-besu \
	helm-redeploy-maru \
	chaos-multi-validator-deploy \
	chaos-multi-validator-deploy-local \
	chaos-multi-validator-full-reload \
	chaos-multi-validator-full-reload-local \

chaos-redeploy-chaos-mesh-and-linea:
	@set -e; \
	$(MAKE) chaos-mesh-reinstall & pid1=$$!; \
	$(MAKE) helm-redeploy-linea maru_image=$(if $(maru_image),$(maru_image)) & pid2=$$!; \
	wait $$pid1 || exit 1; \
	wait $$pid2 || exit 1; \

chaos-reload-k3s-and-chaos-mesh:
	@$(MAKE) k3s-reload
	@$(MAKE) chaos-mesh-install

chaos-full-reload:
	@$(MAKE) k3s-reload
	@$(MAKE) chaos-redeploy-chaos-mesh-and-linea maru_image=$(if $(maru_image),$(maru_image))
	@if [ "$(skip_initial_health_check)" != "true" ]; then \
		$(MAKE) chaos-health-check; \
	else \
		echo "Skipping initial health check"; \
	fi
	@$(MAKE) chaos-experiment-workflow-and-wait # runs workflow-linea-resilience.yaml and waits for it to finish
	@$(MAKE) chaos-health-check

chaos-full-reload-with-local-maru-image:
	@$(MAKE) chaos-full-reload maru_image=consensys/maru:local

chaos-health-check:
	@echo "Running health check for chaos testing environment"
	@$(MAKE) wait-all-running
	@$(MAKE) port-forward-restart-all-linea
	@cd .. && ./gradlew :chaos-testing:health-checker:acceptanceTest --tests "chaos.NodesSyncTest"

chaos-redeploy-linea-and-run-experiment:
	@$(MAKE) helm-redeploy-linea maru_image=$(if $(maru_image),$(maru_image))
	@$(MAKE) chaos-experiment-workflow-and-wait # runs workflow-linea-resilience.yaml and waits for it to finish
	@$(MAKE) chaos-health-check

chaos-experiment-workflow-and-health-check:
	@$(MAKE) chaos-experiment-workflow-and-wait
	@$(MAKE) chaos-health-check

# Multi-validator QBFT baseline targets (no latency injection)
# Usage: make chaos-multi-validator-deploy
# Usage: make chaos-multi-validator-deploy-local   (uses consensys/maru:local image)
# Usage: make chaos-multi-validator-full-reload     (full K3S + chaos-mesh reset first)
# Usage: make chaos-multi-validator-full-reload-local
chaos-multi-validator-deploy:
	@$(MAKE) helm-redeploy-linea-multi-validator maru_image=$(if $(maru_image),$(maru_image))
	@$(MAKE) wait-for-log-entry pod_name=maru-validator-0-0 log_entry="Imported block"
	@$(MAKE) chaos-multi-validator-health-check

chaos-multi-validator-deploy-local:
	@$(MAKE) chaos-multi-validator-deploy maru_image=consensys/maru:local

chaos-multi-validator-full-reload:
	@$(MAKE) k3s-reload
	@$(MAKE) chaos-redeploy-chaos-mesh-and-multi-validator maru_image=$(if $(maru_image),$(maru_image))
	@$(MAKE) wait-for-log-entry pod_name=maru-validator-0-0 log_entry="Imported block"
	@$(MAKE) chaos-multi-validator-health-check

chaos-multi-validator-full-reload-local:
	@$(MAKE) chaos-multi-validator-full-reload maru_image=consensys/maru:local

# Multi-validator QBFT chaos testing targets
chaos-redeploy-chaos-mesh-and-multi-validator:
	@set -e; \
	$(MAKE) chaos-mesh-reinstall & pid1=$$!; \
	$(MAKE) helm-redeploy-linea-multi-validator maru_image=$(if $(maru_image),$(maru_image)) & pid2=$$!; \
	wait $$pid1 || exit 1; \
	wait $$pid2 || exit 1; \

# Fresh validator deploy + single latency level + health check (K3S/chaos-mesh must already be up)
# Usage: make chaos-multi-validator-run-latency-100ms
# Set skip_initial_health_check=true to skip the baseline check (useful when pre-existing issues exist)
chaos-multi-validator-run-latency-%:
	@$(MAKE) helm-redeploy-linea-multi-validator maru_image=$(if $(maru_image),$(maru_image))
	@$(MAKE) wait-for-log-entry pod_name=maru-validator-0-0 log_entry="Imported block"
	@if [ "$(skip_initial_health_check)" != "true" ]; then \
		$(MAKE) chaos-multi-validator-health-check; \
	else \
		echo "Skipping initial health check"; \
	fi
	@$(MAKE) chaos-experiment-multi-validator-latency-$*-and-wait
	@$(MAKE) chaos-multi-validator-health-check

chaos-multi-validator-run-local-latency-%:
	@$(MAKE) chaos-multi-validator-run-latency-$* maru_image=consensys/maru:local

# Full K3S reload + chaos mesh + fresh validators + single latency level + health check
# Usage: make chaos-multi-validator-full-reload-latency-100ms
# Set skip_initial_health_check=true to skip the baseline check (useful when pre-existing issues exist)
chaos-multi-validator-full-reload-latency-%:
	@$(MAKE) k3s-reload
	@$(MAKE) chaos-redeploy-chaos-mesh-and-multi-validator maru_image=$(if $(maru_image),$(maru_image))
	@$(MAKE) wait-for-log-entry pod_name=maru-validator-0-0 log_entry="Imported block"
	@if [ "$(skip_initial_health_check)" != "true" ]; then \
		$(MAKE) chaos-multi-validator-health-check; \
	else \
		echo "Skipping initial health check"; \
	fi
	@$(MAKE) chaos-experiment-multi-validator-latency-$*-and-wait
	@$(MAKE) chaos-multi-validator-health-check

chaos-multi-validator-full-reload-local-latency-%:
	@$(MAKE) chaos-multi-validator-full-reload-latency-$* maru_image=consensys/maru:local

chaos-multi-validator-health-check:
	@echo "Running health check for multi-validator chaos testing environment"
	@$(MAKE) wait-all-running
	@$(MAKE) port-forward-restart-all-linea
	@cd .. && ./gradlew :chaos-testing:health-checker:acceptanceTest --tests "chaos.NodesSyncTest"

