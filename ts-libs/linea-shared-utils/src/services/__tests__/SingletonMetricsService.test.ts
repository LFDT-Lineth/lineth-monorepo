import { Counter, Gauge, Histogram, Registry } from "prom-client";

import { IMetricsService } from "../../core/services/IMetricsService";
import { SingletonMetricsService } from "../SingletonMetricsService";

export enum ExampleMetrics {
  ExampleMetrics = "ExampleMetrics",
}

describe("SingletonMetricsService", () => {
  let metricService: IMetricsService<ExampleMetrics>;

  beforeEach(() => {
    metricService = new SingletonMetricsService({ app: "app" });
  });

  it("should create a counter", () => {
    const counter = metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter");
    expect(counter).toBeInstanceOf(Counter);
  });

  it("should reuse an existing counter instance", () => {
    const counter = metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter");
    const counterAgain = metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter");
    expect(counterAgain).toBe(counter);
  });

  it("should increment a counter", async () => {
    const counter = metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter");
    metricService.incrementCounter(ExampleMetrics.ExampleMetrics, {}, 1);
    expect((await counter.get()).values[0].value).toBe(1);
  });

  it("should increment a counter using default labels and value", async () => {
    metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter");
    metricService.incrementCounter(ExampleMetrics.ExampleMetrics);
    const counterValue = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, {});
    expect(counterValue).toBe(1);
  });

  it("should leave counter untouched when incrementing a missing counter", async () => {
    metricService.incrementCounter(ExampleMetrics.ExampleMetrics, {}, 3);
    const counterValue = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, {});
    expect(counterValue).toBeUndefined();
  });

  it("should create a gauge", () => {
    const gauge = metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    expect(gauge).toBeInstanceOf(Gauge);
  });

  it("should reuse an existing gauge instance", () => {
    const gauge = metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    const gaugeAgain = metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    expect(gaugeAgain).toBe(gauge);
  });

  it("should increment a gauge", async () => {
    const gauge = metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, 5);
    expect((await gauge.get()).values[0].value).toBe(5);
  });

  it("should decrement a gauge", async () => {
    metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, 5);
    metricService.decrementGauge(ExampleMetrics.ExampleMetrics, {}, 2);
    expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(3);
  });

  it("should set a gauge to a specific value", async () => {
    metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    metricService.setGauge(ExampleMetrics.ExampleMetrics, {}, 12);
    expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(12);
  });

  it("should set a gauge when labels are omitted", async () => {
    metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    metricService.setGauge(ExampleMetrics.ExampleMetrics, undefined, 9);
    expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(9);
  });

  it("should increment and decrement gauges using default step", async () => {
    metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics);
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics);
    metricService.decrementGauge(ExampleMetrics.ExampleMetrics);
    expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(1);
  });

  it("should ignore setGauge when gauge does not exist", async () => {
    metricService.setGauge(ExampleMetrics.ExampleMetrics, {}, 7);
    const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
    expect(value).toBeUndefined();
  });

  it("should ignore incrementGauge when gauge does not exist", async () => {
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, 4);
    const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
    expect(value).toBeUndefined();
  });

  it("should ignore decrementGauge when gauge does not exist", async () => {
    metricService.decrementGauge(ExampleMetrics.ExampleMetrics, {}, 2);
    const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
    expect(value).toBeUndefined();
  });

  it("should aggregate gauge values by matching labels", async () => {
    const labelNames = ["status", "region"];
    metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge", labelNames);
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, { status: "ok", region: "us" }, 2);
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, { status: "ok", region: "eu" }, 3);
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, { status: "fail", region: "us" }, 5);

    const aggregatedOk = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, { status: "ok" });
    const aggregatedFail = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, { status: "fail" });
    const missing = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, { status: "missing" });

    expect(aggregatedOk).toBe(5);
    expect(aggregatedFail).toBe(5);
    expect(missing).toBeUndefined();
  });

  it("should aggregate counter values by matching labels", async () => {
    const labelNames = ["status", "region"];
    metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter", labelNames);
    metricService.incrementCounter(ExampleMetrics.ExampleMetrics, { status: "ok", region: "us" }, 4);
    metricService.incrementCounter(ExampleMetrics.ExampleMetrics, { status: "ok", region: "eu" }, 6);

    const aggregated = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, { status: "ok" });
    expect(aggregated).toBe(10);
  });

  it("should return undefined for gauge value when gauge does not exist", async () => {
    const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
    expect(value).toBeUndefined();
  });

  it("should expose the registry instance", () => {
    const registry = (metricService as SingletonMetricsService<ExampleMetrics>).getRegistry();
    expect(registry).toBeInstanceOf(Registry);
  });

  it("should return the correct counter value", async () => {
    metricService.createCounter(ExampleMetrics.ExampleMetrics, "A test counter");
    metricService.incrementCounter(ExampleMetrics.ExampleMetrics, {}, 5);
    const counterValue = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, {});
    expect(counterValue).toBe(5);
  });

  it("should return the correct gauge value", async () => {
    metricService.createGauge(ExampleMetrics.ExampleMetrics, "A test gauge");
    metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, 10);
    const gaugeValue = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
    expect(gaugeValue).toBe(10);
  });

  it("should create a histogram and add values", async () => {
    const histogram = metricService.createHistogram(
      ExampleMetrics.ExampleMetrics,
      [0.1, 0.5, 1, 2, 3, 5],
      "A test histogram",
    );
    expect(histogram).toBeInstanceOf(Histogram);
  });

  it("should add values to histogram and retrieve them", async () => {
    metricService.createHistogram(ExampleMetrics.ExampleMetrics, [0.1, 0.5, 1, 2, 3, 5], "A test histogram");
    metricService.addValueToHistogram(ExampleMetrics.ExampleMetrics, 0.3);
    metricService.addValueToHistogram(ExampleMetrics.ExampleMetrics, 1.5);
    const histogramValues = await metricService.getHistogramMetricsValues(ExampleMetrics.ExampleMetrics);
    expect(histogramValues?.values.length).toBe(9);
  });

  it("should reuse an existing histogram instance", () => {
    const histogram = metricService.createHistogram(ExampleMetrics.ExampleMetrics, [0.1, 0.5], "A test histogram", [
      "status",
    ]);
    const histogramAgain = metricService.createHistogram(
      ExampleMetrics.ExampleMetrics,
      [0.1, 0.5],
      "A test histogram",
      ["status"],
    );
    expect(histogramAgain).toBe(histogram);
  });

  it("should return undefined for histogram metrics when histogram does not exist", async () => {
    const values = await metricService.getHistogramMetricsValues(ExampleMetrics.ExampleMetrics);
    expect(values).toBeUndefined();
  });

  it("should ignore histogram updates when histogram is missing", async () => {
    metricService.addValueToHistogram(ExampleMetrics.ExampleMetrics, 1.2);
    const values = await metricService.getHistogramMetricsValues(ExampleMetrics.ExampleMetrics);
    expect(values).toBeUndefined();
  });
});
