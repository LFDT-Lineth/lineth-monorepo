import { Registry } from "prom-client";
import { mock } from "jest-mock-extended";
import { Api } from "../Api";
import { IMetricsService } from "../../../../core/metrics/IMetricsService";
import { ILogger } from "../../../../core/utils/logging/ILogger";

describe("Api", () => {
  let api: Api;
  const mockConfig = { port: 3000 };
  const mockMetricService = mock<IMetricsService>();
  const mockLogger = mock<ILogger>();

  beforeEach(async () => {
    mockMetricService.getRegistry.mockReturnValue({
      contentType: "text/plain; version=0.0.4; charset=utf-8",
      metrics: async () => "mocked metrics",
    } as Registry);
    api = new Api(mockConfig, mockMetricService, mockLogger);
  });

  afterEach(async () => {
    await api.stop();
  });

  it("should initialize the API", () => {
    expect(api).toBeDefined();
  });

  it("should return metrics from the metric service", async () => {
    await api.start();

    const registry = api["metricsService"].getRegistry();
    expect(registry.contentType).toBe("text/plain; version=0.0.4; charset=utf-8");
    expect(await registry.metrics()).toBe("mocked metrics");
  });

  it("should start the server", async () => {
    await api.start();
    expect(api["server"]).toBeDefined();
  });

  it("should stop the server", async () => {
    await api.start();
    await api.stop();
    expect(api["server"]).toBeUndefined();
  });
});
