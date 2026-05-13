socketPath ?= /run/containerd/containerd.sock
chaos-mesh-install:
	-@helm repo add chaos-mesh https://charts.chaos-mesh.org >/dev/null 2>&1 || true
	@helm repo update
	@echo "socketPath=$(socketPath) KUBECONFIG=$(KUBECONFIG)"
	helm --kubeconfig $(KUBECONFIG) install chaos-mesh chaos-mesh/chaos-mesh \
		--namespace chaos-mesh --create-namespace \
		--version 2.7.2 \
		--set controllerManager.create=true \
		--set controllerManager.clusterScoped=true \
		--set chaosDaemon.create=true \
		--set chaosDaemon.runtime=containerd \
		--set chaosDaemon.socketPath=$(socketPath) \
		--set dashboard.create=true \
		--set dashboard.securityMode=false \
		--set dashboard.service.type=NodePort

chaos-mesh-install-on-k3s:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-install socketPath=/run/k3s/containerd/containerd.sock

chaos-mesh-install-on-aws-eks:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-install socketPath=/run/containerd/containerd.sock

chaos-mesh-uninstall:
	-@kubectl delete ns chaos-mesh --wait=true >/dev/null 2>&1

chaos-mesh-reinstall:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-reinstall-k3s

chaos-mesh-reinstall-k3s:
	-@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-uninstall
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-install-on-k3s

chaos-mesh-reinstall-aws-eks:
	-@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-uninstall
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) chaos-mesh-install-on-aws-eks

chaos-experiment-http-engine-api-error:
	-@kubectl delete -f experiments/http-engine-api-error.yaml --wait-true >/dev/null 2>&1
	@kubectl apply -f experiments/http-engine-api-error.yaml

chaos-experiment-workflow:
	-@kubectl delete -f experiments/workflow-linea-resilience.yaml --wait=true>/dev/null 2>&1
	@kubectl apply -f experiments/workflow-linea-resilience.yaml

experiment_name ?= linea-resilience
experiment_duration ?= 2m
# Converts experiment_duration (2m / 120s / 120) to plain seconds for use with sleep.
experiment_duration_s = $(shell echo '$(experiment_duration)' | awk '/m$$/{sub("m",""); print $$1*60; next} /s$$/{sub("s",""); print $$1; next} {print $$1}')
wait-experiment-done:
	@echo "Waiting for $(experiment_name) workflow to finish..."; \
	timeout=$${WAIT_TIMEOUT_SECONDS:-3000}; interval=$${WAIT_INTERVAL_SECONDS:-10}; elapsed=0; \
	while [ $$elapsed -lt $$timeout ]; do \
		out=$$(kubectl describe workflows.chaos-mesh.org $(experiment_name) -n chaos-mesh 2>/dev/null || true); \
		echo "--- still running, no End Time found yet (elapsed $$elapsed s) ---"; \
		if echo "$$out" | grep -q 'End Time:'; then \
			end_time=$$(echo "$$out" | grep 'End Time:' | head -1); \
			echo "Workflow $(experiment_name) completed: $$end_time"; \
			exit 0; \
		fi; \
		echo "$$out" | grep -qi 'Failed' && { echo "Workflow $(experiment_name) failed."; exit 1; }; \
		sleep $$interval; elapsed=$$((elapsed + interval)); \
	done; \
	echo "Timeout ($$timeout s) waiting for workflow $(experiment_name) completion."; exit 1

chaos-experiment-workflow-and-wait:
	@$(MAKE) chaos-experiment-workflow
	@$(MAKE) wait-experiment-done experiment_name=linea-resilience
	@$(MAKE) wait-all-running

chaos-experiment-multi-validator-latency-%-and-wait:
	# Clean up ALL NetworkChaos resources left from any previous experiment
	-@kubectl --kubeconfig $(KUBECONFIG) delete networkchaos --all \
		-n chaos-mesh --wait=true >/dev/null 2>&1 || true
	# Kill validators to reset in-memory Micrometer metrics to zero
	@kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) delete pods \
		-l app.kubernetes.io/component=maru,app.kubernetes.io/component-role=validator
	# Allow StatefulSet controllers time to create replacement pods before waiting
	@sleep 5
	# Wait for all replacement validator pods to be Ready
	@kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) wait pod \
		-l app.kubernetes.io/component=maru,app.kubernetes.io/component-role=validator \
		--for=condition=ready --timeout=90s
	# Apply NetworkChaos — remains active until deleted after metrics are collected
	@sed 's/__LATENCY__/$*/g' experiments/network-chaos-multi-validator-latency.yaml \
		| kubectl --kubeconfig $(KUBECONFIG) apply -f -
	@echo "Injecting $* latency — experiment window: $(experiment_duration) ($(experiment_duration_s)s)"
	@sleep $(experiment_duration_s)

.PHONY: chaos-mesh-install-with-curl \
	chaos-mesh-install \
	chaos-mesh-uninstall \
	chaos-mesh-run-network-experiment \
	chaos-mesh-reload \
	wait-experiment-done
