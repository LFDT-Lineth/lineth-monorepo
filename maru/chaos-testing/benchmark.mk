.PHONY: chaos-latency-sweep-experiments \
	chaos-multi-validator-local-latency-sweep \
	chaos-multi-validator-local-latency-sweep-fresh \
	chaos-collect-consensus-metrics

# ── Internal helpers ───────────────────────────────────────────────────────────

# Runs the 50→400ms experiment loop on an already-deployed, already-up cluster.
# Not meant to be called directly — use chaos-multi-validator-local-latency-sweep
# or chaos-multi-validator-local-latency-sweep-fresh instead.
chaos-latency-sweep-experiments:
	@echo ""
	@echo "================================================="
	@echo " Baseline (no injected latency)"
	@echo "================================================="
	@$(MAKE) chaos-multi-validator-baseline-experiment experiment_duration=$${experiment_duration:-5m}
	@for ms in 50 100 150 200 250 300 350 400; do \
		echo ""; \
		echo "================================================="; \
		echo " Latency experiment: $${ms}ms"; \
		echo "================================================="; \
		$(MAKE) chaos-multi-validator-latency-experiment-$${ms}ms experiment_duration=$${experiment_duration:-5m}; \
	done
	@echo ""
	@echo "=== Sweep complete: baseline 50ms 100ms 150ms 200ms 250ms 300ms 350ms 400ms ==="

# ── Single experiment ──────────────────────────────────────────────────────────

# Run a baseline experiment (no injected latency) on an already-running cluster.
# Validators are restarted to reset metrics, then blocks are produced at natural latency.
# Usage: make chaos-multi-validator-baseline-experiment [experiment_duration=10m]
chaos-multi-validator-baseline-experiment:
	@kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) delete pods \
		-l app.kubernetes.io/component=maru,app.kubernetes.io/component-role=validator
	@sleep 5
	@kubectl --kubeconfig $(KUBECONFIG) -n $(NAMESPACE) wait pod \
		-l app.kubernetes.io/component=maru,app.kubernetes.io/component-role=validator \
		--for=condition=ready --timeout=90s
	@echo "Baseline (no latency) — experiment window: $(experiment_duration) ($(experiment_duration_s)s)"
	@sleep $(experiment_duration_s)
	@$(MAKE) chaos-collect-consensus-metrics experiment_latency=baseline

# Run a single latency experiment on an already-running cluster (no redeploy).
# Validators are restarted by the chaos workflow at the start, resetting metrics to zero.
# All blocks committed during the experiment are produced under the injected latency.
# Usage: make chaos-multi-validator-latency-experiment-100ms [experiment_duration=10m]
chaos-multi-validator-latency-experiment-%:
	@$(MAKE) chaos-experiment-multi-validator-latency-$*-and-wait
	@$(MAKE) chaos-collect-consensus-metrics experiment_latency=$*
	-@kubectl --kubeconfig $(KUBECONFIG) delete networkchaos multi-validator-latency-$* -n chaos-mesh --wait=false >/dev/null 2>&1 || true

# ── Sweep: idempotent, works from any state ───────────────────────────────────

# Full reset (K3S + chaos-mesh + local maru build + deploy), then run latency sweep.
# Idempotent: creates K3S if missing, resets if already running.
# Usage: make chaos-multi-validator-local-latency-sweep [experiment_duration=10m]
chaos-multi-validator-local-latency-sweep:
	@$(MAKE) chaos-multi-validator-full-reload-local
	@$(MAKE) chaos-latency-sweep-experiments

# ── Metrics collection ────────────────────────────────────────────────────────

# Collect QBFT consensus latency metrics from all maru pods and print aggregated stats.
# Requires port-forwards to be active (done automatically by port-forward-restart-all-linea).
# Optional: experiment_latency=100ms  — labels the report and saves output to
#   tmp/results/experiment-100ms.log  (tmp/results/experiment-unlabeled.log if unset).
# Usage:
#   make chaos-collect-consensus-metrics
#   make chaos-collect-consensus-metrics experiment_latency=100ms
chaos-collect-consensus-metrics:
	@$(MAKE) port-forward-restart-all-linea
	@mkdir -p tmp/results
	@label=$${experiment_latency:-unlabeled}; \
	cd .. && ./gradlew :chaos-testing:health-checker:acceptanceTest \
		--tests "chaos.ConsensusMetricsBenchmarkTest" \
		$(if $(experiment_latency),-Dexperiment.latency=$(experiment_latency),) \
		2>&1 | tee chaos-testing/tmp/results/experiment-$$label.log
