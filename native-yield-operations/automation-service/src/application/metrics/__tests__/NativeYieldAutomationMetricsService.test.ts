import { describe, it, expect } from "@jest/globals";

import { NativeYieldAutomationMetricsService } from "../NativeYieldAutomationMetricsService.js";
import { LineaNativeYieldAutomationServiceMetrics } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

// Test constants
const DEFAULT_APP_LABEL = 'app="native-yield-automation-service"';
const CUSTOM_SERVICE_LABEL = 'service="custom"';

const createMetricsService = (defaultLabels?: Record<string, string>): NativeYieldAutomationMetricsService => {
  return new NativeYieldAutomationMetricsService(defaultLabels);
};

describe("NativeYieldAutomationMetricsService", () => {
  describe("default labels", () => {
    it("applies default labels when none are provided", async () => {
      // Arrange
      const service = createMetricsService();
      const counter = service.createCounter(
        LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal,
        "test counter",
      );

      // Act
      counter.inc();
      const metricsOutput = await service.getRegistry().metrics();

      // Assert
      expect(metricsOutput).toContain(DEFAULT_APP_LABEL);
    });

    it("overrides default labels when custom labels are provided", async () => {
      // Arrange
      const service = createMetricsService({ service: "custom" });
      const counter = service.createCounter(
        LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal,
        "custom label counter",
      );

      // Act
      counter.inc();
      const metricsOutput = await service.getRegistry().metrics();

      // Assert
      expect(metricsOutput).toContain(CUSTOM_SERVICE_LABEL);
      expect(metricsOutput).not.toContain(DEFAULT_APP_LABEL);
    });
  });
});
