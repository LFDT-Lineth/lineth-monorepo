.SILENT:

export PATH := /usr/local/bin:$(PATH)

KUBECONFIG       ?= ~/.kube/k3s-server
NAMESPACE        ?= default
# Optional extra --set args appended to every helm upgrade --install call.
# Example: HELM_EXTRA_ARGS='--set nodeSelector."kubernetes\.io/arch"=amd64'
HELM_EXTRA_ARGS  ?=

helm-clean-pvcs:
	@echo "Cleaning up $(component) Persistent Volumes Claims and correspondent PV"
	-@kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) delete pvc -l app.kubernetes.io/component=$(component) >/dev/null 2>&1
	-@kubectl --kubeconfig $(KUBECONFIG) get pv --no-headers 2>/dev/null | awk '$$5=="Available" {print $$1}' | xargs -r -I {} kubectl --kubeconfig $(KUBECONFIG) delete pv {} >/dev/null 2>&1

helm-clean-component:
	@if [ -z "$(component)" ]; then \
		echo "Usage: make helm-clean-component component=<component_name>"; \
		exit 1; \
	fi
	@echo "Cleaning up all $(component) releases"
	@COMPONENT_RELEASES=$$(helm --kubeconfig $(KUBECONFIG) list -q --namespace $(NAMESPACE) 2>/dev/null | grep '^$(component)-' || true); \
	if [ -n "$$COMPONENT_RELEASES" ]; then \
		for release in $$COMPONENT_RELEASES; do \
			echo "Uninstalling $$release"; \
			helm --kubeconfig $(KUBECONFIG) uninstall $$release --namespace $(NAMESPACE) >/dev/null 2>&1 || true; \
		done; \
	else \
		echo "No $(component) releases found"; \
	fi
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-pvcs component=$(component)

helm-clean-besu-releases:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-component component=besu

helm-clean-maru-releases:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-component component=maru

helm-clean-linea-releases:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-besu-releases
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-maru-releases

wait_pods:
	@if [ -z "$(pod_name)" ] || [ -z "$(pod_count)" ]; then \
		echo "Usage: make wait_pods pod_name=<pod_name> pod_count=<pod_count>"; \
		exit 1; \
	fi; \
	echo "Waiting for $(pod_count) pods with label app.kubernetes.io/pod-name=$(pod_name) to be up and running..."; \
	while true; do \
		current_count=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) get pods -l app.kubernetes.io/name=$(pod_name) --field-selector=status.phase=Running -o json | jq '.items | length'); \
		if [ "$$current_count" -ge "$(pod_count)" ]; then \
			echo "$$current_count $$pod_name pods are up and running."; \
			break; \
		fi; \
		echo "$$current_count $$pod_name pods are running. Waiting..."; \
		sleep 5; \
	done

wait-for-log-entry:
	@until kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) logs $(pod_name) | grep -q "$(log_entry)"; do \
		sleep 1; \
	done


helm-deploy-maru-only-node:
	@if [ -z "$(maru_release_name)" ] || [ -z "$(maru_values)" ]; then \
		echo "Usage: make helm-deploy-maru-only-node maru_release_name=<name> maru_values=<values_file> [maru_bootnode=<bootnode>]"; \
		exit 1; \
	fi
	@echo "Deploying Maru-only node: $(maru_release_name)"
	@MARU_ARGS=""; \
	if [ -n "$(maru_bootnode)" ]; then \
		MARU_ARGS="--set bootnodes=$(maru_bootnode)"; \
	fi; \
	if [ -n "$(maru_image)" ]; then \
		MARU_ARGS="$$MARU_ARGS --set image.name=$(maru_image)"; \
	fi; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install $(maru_release_name) ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/$(maru_values) --namespace $(NAMESPACE) --create-namespace $$MARU_ARGS $(HELM_EXTRA_ARGS)

# make helm-deploy-linea-node maru_release_name=maru-bootnode-0 maru_values=maru-bootnode-0.yaml besu_release_name=besu-bootnode-0 besu_values=besu-bootnode-0.yaml
# to debug templates:
# helm template maru-test ./helm/charts/maru -f ./helm/charts/maru/values.yaml -f ./helm/values/maru-bootnode-0.yaml --debug
helm-deploy-linea-node:
	@if [ -z "$(maru_release_name)" ] || [ -z "$(maru_values)" ] || [ -z "$(besu_release_name)" ] || [ -z "$(besu_values)" ]; then \
		echo "Usage: make helm-deploy-linea-node maru_release_name=<name> maru_values=<values_file> besu_release_name=<name> besu_values=<values_file> [maru_bootnode=<bootnode>] [besu_bootnode=<bootnode>]"; \
		exit 1; \
	fi
	@echo "Deploying Linea node - Besu: $(besu_release_name), Maru: $(maru_release_name)"
	@BESU_ARGS=""; \
	if [ -n "$(besu_bootnode)" ]; then \
		BESU_ARGS="--set bootnodes=$(besu_bootnode)"; \
	fi; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install $(besu_release_name) ./helm/charts/besu --force -f ./helm/charts/besu/values.yaml -f ./helm/values/$(besu_values) --namespace $(NAMESPACE) --create-namespace $$BESU_ARGS $(HELM_EXTRA_ARGS)
	@kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) wait --for=condition=ready pod/$(besu_release_name)-0 --timeout=120s
	@MARU_ARGS=""; \
	if [ -n "$(maru_bootnode)" ]; then \
		MARU_ARGS="--set bootnodes=$(maru_bootnode)"; \
	fi; \
	if [ -n "$(maru_image)" ]; then \
		MARU_ARGS="$$MARU_ARGS --set image.name=$(maru_image)"; \
	fi; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install $(maru_release_name) ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/$(maru_values) --namespace $(NAMESPACE) --create-namespace $$MARU_ARGS $(HELM_EXTRA_ARGS)


get-pod-enode:
	@mkdir -p tmp
	@$(MAKE) wait-for-log-entry pod_name=$(pod_name) log_entry="enode"
	@BOOTNODE_IP=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) describe pod $(pod_name) | grep -E '^IP:' | awk '{print $$2}'); \
	ENODE=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) logs $(pod_name) | grep -o 'enode://[^[:space:]]*' | sed "s/127\.0\.0\.1/$$BOOTNODE_IP/"); \
	echo "ENODE: $$ENODE"; \
	DST_FILE=$${dst_file:-enode-$(pod_name).txt}; \
	echo "$$ENODE" > "tmp/$$DST_FILE"; \
	echo "ENODE saved to tmp/$$DST_FILE"

get-pod-enr:
	@mkdir -p tmp
	@$(MAKE) wait-for-log-entry pod_name=$(pod_name) log_entry="enr"
	ENR=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) logs $(pod_name) | grep -Ev '0.0.0.0|127.0.0.1' | grep -o 'enr=[^ ]*' | head -1 | cut -d= -f2); \
	echo "ENR: $$ENR"; \
	DST_FILE=$${dst_file:-enr-$(pod_name).txt}; \
	echo "$$ENR" > "tmp/$$DST_FILE"; \
	echo "ENR saved to tmp/$$DST_FILE"

IMAGE_ARG=$(if $(maru_image),--set image.name=$(maru_image),)
helm-redeploy-maru:
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-maru-releases
	@echo "Deploying Maru Nodes: IMAGE_ARG='$(IMAGE_ARG)'"
	@helm --kubeconfig $(KUBECONFIG) upgrade --install maru-bootnode-0 ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/maru-bootnode-0.yaml --namespace $(NAMESPACE) --create-namespace $(IMAGE_ARG) $(HELM_EXTRA_ARGS);
	$(MAKE) get-pod-enr pod_name=maru-bootnode-0-0 dst_file=maru-bootnode.txt; \
	MARU_BOOTNODE=$$(cat tmp/maru-bootnode.txt); \
	echo "Deploying remaining nodes with $$MARU_BOOTNODE"; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install maru-sequencer-0 ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/maru-sequencer-0.yaml --namespace $(NAMESPACE) --set bootnodes=$$BOOTNODE_ENR $(IMAGE_ARG) $(HELM_EXTRA_ARGS) ; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install maru-follower-1  ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/maru-follower-1.yaml  --namespace $(NAMESPACE) --set bootnodes=$$BOOTNODE_ENR $(IMAGE_ARG) $(HELM_EXTRA_ARGS) ; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install maru-follower-2  ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/maru-follower-2.yaml  --namespace $(NAMESPACE) --set bootnodes=$$BOOTNODE_ENR $(IMAGE_ARG) $(HELM_EXTRA_ARGS) ; \
	helm --kubeconfig $(KUBECONFIG) upgrade --install maru-follower-3  ./helm/charts/maru --force -f ./helm/charts/maru/values.yaml -f ./helm/values/maru-follower-3.yaml  --namespace $(NAMESPACE) --set bootnodes=$$BOOTNODE_ENR $(IMAGE_ARG) $(HELM_EXTRA_ARGS)
	@MAKE wait_pods pod_name=maru-bootnode-0 pod_count=1
	@$(MAKE) wait-all-running

helm-redeploy-linea:
	$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-linea-releases NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@set -e; \
	pid1=""; \
	if [ "$(maru_image)" ]; then \
		case "$(maru_image)" in \
			*local) \
				echo "maru_image ends with 'local' -> build/import local image"; \
				$(MAKE) build-and-import-maru-image & pid1=$$!; \
				;; \
			*) \
				echo "Using provided maru_image=$(maru_image)"; \
				;; \
		esac; \
	fi; \
	if [ -n "$$pid1" ]; then wait $$pid1 || exit 1; fi;
	@$(MAKE) helm-deploy-linea-node maru_release_name=maru-bootnode-0 maru_values=maru-bootnode-0.yaml besu_release_name=besu-bootnode-0 besu_values=besu-bootnode-0.yaml NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG);
	@$(MAKE) get-pod-enode pod_name=besu-bootnode-0-0 dst_file=el-bootnode.txt NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@$(MAKE) get-pod-enr pod_name=maru-bootnode-0-0 dst_file=maru-bootnode.txt NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@EL_BOOTNODE=$$(cat tmp/el-bootnode.txt); \
	MARU_BOOTNODE=$$(cat tmp/maru-bootnode.txt); \
	echo "Deploying remaining nodes with bootnodes - EL: $$EL_BOOTNODE, Maru: $$MARU_BOOTNODE"; \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-sequencer-0 maru_values=maru-sequencer-0.yaml besu_release_name=besu-sequencer-0 besu_values=besu-sequencer-0.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-follower-1 maru_values=maru-follower-1.yaml besu_release_name=besu-follower-1 besu_values=besu-follower-1.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-follower-2 maru_values=maru-follower-2.yaml besu_release_name=besu-follower-2 besu_values=besu-follower-2.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-follower-3 maru_values=maru-follower-3.yaml besu_release_name=besu-follower-3 besu_values=besu-follower-3.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	sleep 3; \
	$(MAKE) wait-all-running NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	echo "Deployment done"

helm-redeploy-linea-multi-validator:
	$(MAKE) -f $(firstword $(MAKEFILE_LIST)) helm-clean-linea-releases NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@set -e; \
	pid1=""; \
	if [ "$(maru_image)" ]; then \
		case "$(maru_image)" in \
			*local) \
				echo "maru_image ends with 'local' -> build/import local image"; \
				$(MAKE) build-and-import-maru-image & pid1=$$!; \
				;; \
			*) \
				echo "Using provided maru_image=$(maru_image)"; \
				;; \
		esac; \
	fi; \
	if [ -n "$$pid1" ]; then wait $$pid1 || exit 1; fi;
	@$(MAKE) helm-deploy-maru-only-node maru_release_name=maru-bootnode-0 maru_values=maru-bootnode-mv-0.yaml $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG);
	@$(MAKE) get-pod-enr pod_name=maru-bootnode-0-0 dst_file=maru-bootnode.txt NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@MARU_BOOTNODE=$$(cat tmp/maru-bootnode.txt); \
	echo "Deploying validator-0 with Maru bootnode: $$MARU_BOOTNODE"; \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-validator-0 maru_values=maru-validator-0.yaml besu_release_name=besu-validator-0 besu_values=besu-validator-0.yaml maru_bootnode=$$MARU_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@$(MAKE) get-pod-enode pod_name=besu-validator-0-0 dst_file=el-bootnode.txt NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG)
	@EL_BOOTNODE=$$(cat tmp/el-bootnode.txt); \
	MARU_BOOTNODE=$$(cat tmp/maru-bootnode.txt); \
	echo "Deploying validators 1-3 with bootnodes - EL: $$EL_BOOTNODE, Maru: $$MARU_BOOTNODE"; \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-validator-1 maru_values=maru-validator-1.yaml besu_release_name=besu-validator-1 besu_values=besu-validator-1.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-validator-2 maru_values=maru-validator-2.yaml besu_release_name=besu-validator-2 besu_values=besu-validator-2.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	$(MAKE) helm-deploy-linea-node maru_release_name=maru-validator-3 maru_values=maru-validator-3.yaml besu_release_name=besu-validator-3 besu_values=besu-validator-3.yaml maru_bootnode=$$MARU_BOOTNODE besu_bootnode=$$EL_BOOTNODE $(if $(maru_image),maru_image=$(maru_image),) NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	sleep 3; \
	$(MAKE) wait-all-running NAMESPACE=$(NAMESPACE) KUBECONFIG=$(KUBECONFIG); \
	echo "Multi-validator deployment done"

build-maru-image:
	@echo "Building Maru image"
	cd .. && ./gradlew :app:installDist
	cd .. && docker build app --build-context=libs=./app/build/install/app/lib/ -t consensys/maru:local

build-and-import-maru-image:
	@$(MAKE) build-maru-image
	@$(MAKE) k3s-import-local-maru-image

build-and-redeploy-maru:
	@$(MAKE) build-and-import-maru-image
	@$(MAKE) helm-redeploy-maru

# Port-forward component pods exposing each pod's <port> on incremental local ports
# Usage:
#   make port-forward-all component=besu port=8545          -> 1000, 1001, ...
#   make port-forward-all component=maru port=8550          -> 1000, 1001, ...
# Optional vars: local_port_start_number (default 1000)
# Backward compatibility: besu-port-forward-all still works (defaults component=besu remote_port=8545)
component ?= besu
port ?= 8545
local_port_start_number ?= 1000
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
TMP_DIR := $(MAKEFILE_DIR)tmp
TMP_DIR_PF := $(TMP_DIR)/pf
port-forward-component:
	mkdir -p $(TMP_DIR_PF); \
	echo "Using dirs: $(TMP_DIR) $(TMP_DIR_PF)"; \
	summary_file="$(TMP_DIR)/port-forward-$(component)-$(port).txt"; \
	: > "$$summary_file"; \
	echo "Discovering pods (label app.kubernetes.io/component=$(component))..."; \
	pods=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) get pods -l app.kubernetes.io/component=$(component) -o jsonpath='{.items[*].metadata.name}'); \
	if [ -z "$$pods" ]; then echo "No pods found for component $(component)"; exit 1; fi; \
	current_port=$(local_port_start_number); \
	for pod in $$pods; do \
		while ss -tln 2>/dev/null | grep -q ":$$current_port " || nc -z 127.0.0.1 $$current_port 2>/dev/null; do \
			echo "Local port $$current_port in use, trying next..."; \
			current_port=$$((current_port + 1)); \
		done; \
		log_file="$(TMP_DIR_PF)/port-forward-$$pod-$(port).log"; \
		pid_file="$(TMP_DIR_PF)/port-forward-$$pod-$(port).pid"; \
		kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) port-forward $$pod $$current_port:$(port) > "$$log_file" 2>&1 & \
		pf_pid=$$!; \
		echo $$pf_pid > "$$pid_file"; \
		retries=0; \
		while ! grep -q "Forwarding from" "$$log_file" 2>/dev/null; do \
			if ! kill -0 $$pf_pid 2>/dev/null; then \
				echo "ERROR: port-forward process died for $$pod on port $$current_port. Log:"; \
				cat "$$log_file"; \
				exit 1; \
			fi; \
			retries=$$((retries + 1)); \
			if [ $$retries -ge 30 ]; then \
				echo "ERROR: port-forward for $$pod not listening on $$current_port after 30s. Log:"; \
				cat "$$log_file"; \
				exit 1; \
			fi; \
			sleep 1; \
		done; \
		url="$$pod = http://127.0.0.1:$$current_port"; \
		url_log="$$pod:$(port) -> http://127.0.0.1:$$current_port (pid=$$pf_pid)"; \
		echo "$$url" >> "$$summary_file"; \
		echo "$$url_log"; \
		current_port=$$((current_port + 1)); \
	done; \
	echo "URL list written to $$summary_file";

port-forward-linea:
	$(MAKE) port-forward-component component=maru port=5060 local_port_start_number=1100
	$(MAKE) port-forward-component component=maru port=9545 local_port_start_number=1150
	$(MAKE) port-forward-component component=besu port=8545 local_port_start_number=1200

port-forward-stop-component:
	@echo "Scanning for all kubectl port-forward processes..."; \
	pids=$$(ps -o pid= -o command= -ax | grep 'kubectl port-forward' | grep -v grep | grep $(component) | awk '{print $$1}'); \
	if [ -z "$$pids" ]; then \
		echo "No kubectl port-forward processes found."; \
	else \
		for pid in $$pids; do \
			if kill -0 $$pid >/dev/null 2>&1; then \
				echo "Killing PID $$pid"; \
				kill $$pid 2>/dev/null || echo "Failed to kill $$pid with SIGTERM"; \
			fi; \
		done; \
		sleep 1; \
		for pid in $$pids; do \
			if kill -0 $$pid >/dev/null 2>&1; then \
				echo "Force killing PID $$pid"; \
				kill -9 $$pid 2>/dev/null || echo "Failed to force kill $$pid"; \
			fi; \
		done; \
	fi; \
	echo "Removing tracked PID files in $(TMP_DIR_PF)"; \
	rm -f $(TMP_DIR_PF)/port-forward-$(component)*.pid 2>/dev/null || true; \
	rm -f $(TMP_DIR_PF)/port-forward-$(component)*.log 2>/dev/null || true; \
	echo "Done.";

port-forward-stop-all-linea:
	$(MAKE) port-forward-stop-component component=maru;
	$(MAKE) port-forward-stop-component component=besu;

port-forward-restart-all-linea:
	-$(MAKE) port-forward-stop-all-linea
	$(MAKE) port-forward-linea

wait-all-running:
	@uptime_arg="$${uptime:-30s}"; \
	echo "Waiting for all pods in $(NAMESPACE) namespace to be running for at least $$uptime_arg since last restart..."; \
	uptime_seconds=0; \
	case "$$uptime_arg" in \
		*s) uptime_seconds=$$(echo "$$uptime_arg" | sed 's/s$$//'); ;; \
		*m) uptime_seconds=$$(echo "$$uptime_arg" | sed 's/m$$//' | awk '{print $$1 * 60}'); ;; \
		*h) uptime_seconds=$$(echo "$$uptime_arg" | sed 's/h$$//' | awk '{print $$1 * 3600}'); ;; \
		*) echo "Invalid uptime format. Use format like: 30s, 2m, 1h"; exit 1; ;; \
	esac; \
	while true; do \
		total_pods=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | wc -l | tr -d ' '); \
		pods_data=$$(kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) get pods --field-selector=status.phase=Running -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[*].state.running.startedAt}{"\n"}{end}' 2>/dev/null); \
		if [ -z "$$pods_data" ] || [ $$(echo "$$pods_data" | wc -l | tr -d ' ') -lt "$$total_pods" ]; then \
			echo "Not all pods are running in $(NAMESPACE) namespace. Waiting..."; \
			sleep 2; \
			continue; \
		fi; \
		pods_ready=0; \
		current_time=$$(date +%s); \
		for line in $$pods_data; do \
			if [ -z "$$line" ]; then continue; fi; \
			pod_name=$$(echo "$$line" | awk '{print $$1}'); \
			start_times=$$(echo "$$line" | cut -d' ' -f2-); \
			if [ -z "$$pod_name" ]; then continue; fi; \
			most_recent_seconds=0; \
			for start_time in $$start_times; do \
				if [ -n "$$start_time" ]; then \
					start_time_clean=$$(echo "$$start_time" | sed 's/\.[0-9]*Z$$/Z/'); \
					if date --version >/dev/null 2>&1; then \
						start_seconds=$$(date -d "$$start_time_clean" +%s 2>/dev/null || echo "0"); \
					else \
						start_seconds=$$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$$start_time_clean" "+%s" 2>/dev/null || echo "0"); \
					fi; \
					if [ "$$start_seconds" -gt "$$most_recent_seconds" ]; then \
						most_recent_seconds=$$start_seconds; \
					fi; \
				fi; \
			done; \
			if [ "$$most_recent_seconds" -gt 0 ]; then \
				container_uptime=$$((current_time - most_recent_seconds)); \
				if [ "$$container_uptime" -ge "$$uptime_seconds" ]; then \
					pods_ready=$$((pods_ready + 1)); \
				else \
					echo "Pod $$pod_name: uptime $$container_uptime seconds < required $$uptime_seconds seconds"; \
				fi; \
			else \
				echo "Pod $$pod_name: Could not parse container start time, skipping"; \
			fi; \
		done; \
		if [ "$$pods_ready" -eq "$$total_pods" ]; then \
			echo "All $$total_pods pods have been running for at least $$uptime_seconds seconds since last restart."; \
			break; \
		fi; \
		echo "$$pods_ready/$$total_pods pods have been running for at least $$uptime_seconds seconds since last restart. Waiting..."; \
		sleep 2; \
	done
