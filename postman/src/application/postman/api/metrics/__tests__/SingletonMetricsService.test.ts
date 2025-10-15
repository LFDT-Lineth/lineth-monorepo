import { Counter, Gauge, Histogram } from "prom-client";
import { IMetricsService, LineaPostmanMetrics } from "../../../../../core/metrics/IMetricsService";
import { SingletonMetricsService } from "../SingletonMetricsService";

describe("SingletonMetricsService", () => {
  let metricService: IMetricsService;

  beforeEach(() => {
    metricService = new SingletonMetricsService();
  });

  it("should create a counter", () => {
    const counter = metricService.createCounter(LineaPostmanMetrics.Messages, "A test counter");
    expect(counter).toBeInstanceOf(Counter);
  });

  it("should increment a counter", async () => {
    const counter = metricService.createCounter(LineaPostmanMetrics.Messages, "A test counter");
    metricService.incrementCounter(LineaPostmanMetrics.Messages, {}, 1);
    expect((await counter.get()).values[0].value).toBe(1);
  });

  it("should create a gauge", () => {
    const gauge = metricService.createGauge(LineaPostmanMetrics.Messages, "A test gauge");
    expect(gauge).toBeInstanceOf(Gauge);
  });

  it("should increment a gauge", async () => {
    const gauge = metricService.createGauge(LineaPostmanMetrics.Messages, "A test gauge");
    metricService.incrementGauge(LineaPostmanMetrics.Messages, {}, 5);
    expect((await gauge.get()).values[0].value).toBe(5);
  });

  it("should decrement a gauge", async () => {
    metricService.createGauge(LineaPostmanMetrics.Messages, "A test gauge");
    metricService.incrementGauge(LineaPostmanMetrics.Messages, {}, 5);
    metricService.decrementGauge(LineaPostmanMetrics.Messages, {}, 2);
    expect(await metricService.getGaugeValue(LineaPostmanMetrics.Messages, {})).toBe(3);
  });

  it("should return the correct counter value", async () => {
    metricService.createCounter(LineaPostmanMetrics.Messages, "A test counter");
    metricService.incrementCounter(LineaPostmanMetrics.Messages, {}, 5);
    const counterValue = await metricService.getCounterValue(LineaPostmanMetrics.Messages, {});
    expect(counterValue).toBe(5);
  });

  it("should return the correct gauge value", async () => {
    metricService.createGauge(LineaPostmanMetrics.Messages, "A test gauge");
    metricService.incrementGauge(LineaPostmanMetrics.Messages, {}, 10);
    const gaugeValue = await metricService.getGaugeValue(LineaPostmanMetrics.Messages, {});
    expect(gaugeValue).toBe(10);
  });

  it("should create a histogram and add values", async () => {
    const histogram = metricService.createHistogram(
      LineaPostmanMetrics.TransactionProcessingTime,
      [0.1, 0.5, 1, 2, 3, 5],
      "A test histogram",
    );
    expect(histogram).toBeInstanceOf(Histogram);
  });

  it("should add values to histogram and retrieve them", async () => {
    metricService.createHistogram(
      LineaPostmanMetrics.TransactionProcessingTime,
      [0.1, 0.5, 1, 2, 3, 5],
      "A test histogram",
    );
    metricService.addValueToHistogram(LineaPostmanMetrics.TransactionProcessingTime, 0.3);
    metricService.addValueToHistogram(LineaPostmanMetrics.TransactionProcessingTime, 1.5);
    const histogramValues = await metricService.getHistogramMetricsValues(
      LineaPostmanMetrics.TransactionProcessingTime,
    );
    expect(histogramValues?.values.length).toBe(9);
  });
});
