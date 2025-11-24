import { SingletonMetricsService } from "@consensys/linea-shared-utils";
import { LineaNativeYieldAutomationServiceMetrics } from "../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

/**
 * Metrics service for the Native Yield Automation Service.
 * Extends SingletonMetricsService to provide singleton access to metrics collection
 * with service-specific default labels.
 */
export class NativeYieldAutomationMetricsService extends SingletonMetricsService<LineaNativeYieldAutomationServiceMetrics> {
  /**
   * Creates a new NativeYieldAutomationMetricsService instance.
   *
   * @param {Record<string, string>} [defaultLabels={ app: "native-yield-automation-service" }] - Default labels to apply to all metrics collected by this service.
   */
  constructor(defaultLabels: Record<string, string> = { app: "native-yield-automation-service" }) {
    super(defaultLabels);
  }
}
