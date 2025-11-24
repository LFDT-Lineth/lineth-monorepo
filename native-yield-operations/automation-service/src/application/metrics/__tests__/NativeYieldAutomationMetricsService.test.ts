import { NativeYieldAutomationMetricsService } from "../NativeYieldAutomationMetricsService.js";
import { LineaNativeYieldAutomationServiceMetrics } from "../../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

describe("NativeYieldAutomationMetricsService", () => {
  const TEST_METRIC = LineaNativeYieldAutomationServiceMetrics.ReportYieldTotal;

  it("applies default labels when none are provided", async () => {
    const service = new NativeYieldAutomationMetricsService();
    const counter = service.createCounter(TEST_METRIC, "test counter");

    counter.inc();

    const metricsOutput = await service.getRegistry().metrics();
    expect(metricsOutput).toContain('app="native-yield-automation-service"');
  });

  it("allows overriding default labels through constructor parameter", async () => {
    const service = new NativeYieldAutomationMetricsService({ service: "custom" });
    const counter = service.createCounter(TEST_METRIC, "custom label counter");

    counter.inc();

    const metricsOutput = await service.getRegistry().metrics();
    expect(metricsOutput).toContain('service="custom"');
    expect(metricsOutput).not.toContain('app="native-yield-automation-service"');
  });
});
