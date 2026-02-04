import { Counter, Gauge, Histogram, Registry } from "prom-client";

import { IMetricsService } from "../../core/services/IMetricsService";
import { SingletonMetricsService } from "../SingletonMetricsService";

export enum ExampleMetrics {
  ExampleMetrics = "ExampleMetrics",
}

// Test constants
const TEST_APP_NAME = "app";
const TEST_COUNTER_DESCRIPTION = "A test counter";
const TEST_GAUGE_DESCRIPTION = "A test gauge";
const TEST_HISTOGRAM_DESCRIPTION = "A test histogram";
const HISTOGRAM_BUCKETS = [0.1, 0.5, 1, 2, 3, 5];
const HISTOGRAM_BUCKETS_SMALL = [0.1, 0.5];
const DEFAULT_INCREMENT = 1;
const EXPECTED_HISTOGRAM_VALUES_LENGTH = 9;

describe("SingletonMetricsService", () => {
  let metricService: IMetricsService<ExampleMetrics>;

  beforeEach(() => {
    metricService = new SingletonMetricsService({ app: TEST_APP_NAME });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("Counter Operations", () => {
    it("create a counter instance", () => {
      // Arrange
      // (no setup needed)

      // Act
      const counter = metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION);

      // Assert
      expect(counter).toBeInstanceOf(Counter);
    });

    it("reuse existing counter when creating duplicate", () => {
      // Arrange
      const counter = metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION);

      // Act
      const counterAgain = metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION);

      // Assert
      expect(counterAgain).toBe(counter);
    });

    it("increment counter with specified value", async () => {
      // Arrange
      const counter = metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION);
      const incrementValue = 1;

      // Act
      metricService.incrementCounter(ExampleMetrics.ExampleMetrics, {}, incrementValue);

      // Assert
      expect((await counter.get()).values[0].value).toBe(incrementValue);
    });

    it("increment counter with default labels and value", async () => {
      // Arrange
      metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION);

      // Act
      metricService.incrementCounter(ExampleMetrics.ExampleMetrics);

      // Assert
      const counterValue = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, {});
      expect(counterValue).toBe(DEFAULT_INCREMENT);
    });

    it("return undefined when incrementing non-existent counter", async () => {
      // Arrange
      const incrementValue = 3;

      // Act
      metricService.incrementCounter(ExampleMetrics.ExampleMetrics, {}, incrementValue);

      // Assert
      const counterValue = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, {});
      expect(counterValue).toBeUndefined();
    });

    it("return counter value after increment", async () => {
      // Arrange
      metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION);
      const incrementValue = 5;

      // Act
      metricService.incrementCounter(ExampleMetrics.ExampleMetrics, {}, incrementValue);

      // Assert
      const counterValue = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, {});
      expect(counterValue).toBe(incrementValue);
    });

    it("aggregate counter values across matching labels", async () => {
      // Arrange
      const labelNames = ["status", "region"];
      metricService.createCounter(ExampleMetrics.ExampleMetrics, TEST_COUNTER_DESCRIPTION, labelNames);
      const usIncrement = 4;
      const euIncrement = 6;
      const expectedAggregate = usIncrement + euIncrement;

      // Act
      metricService.incrementCounter(ExampleMetrics.ExampleMetrics, { status: "ok", region: "us" }, usIncrement);
      metricService.incrementCounter(ExampleMetrics.ExampleMetrics, { status: "ok", region: "eu" }, euIncrement);

      // Assert
      const aggregated = await metricService.getCounterValue(ExampleMetrics.ExampleMetrics, { status: "ok" });
      expect(aggregated).toBe(expectedAggregate);
    });
  });

  describe("Gauge Operations", () => {
    it("create a gauge instance", () => {
      // Arrange
      // (no setup needed)

      // Act
      const gauge = metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);

      // Assert
      expect(gauge).toBeInstanceOf(Gauge);
    });

    it("reuse existing gauge when creating duplicate", () => {
      // Arrange
      const gauge = metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);

      // Act
      const gaugeAgain = metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);

      // Assert
      expect(gaugeAgain).toBe(gauge);
    });

    it("increment gauge with specified value", async () => {
      // Arrange
      const gauge = metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);
      const incrementValue = 5;

      // Act
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, incrementValue);

      // Assert
      expect((await gauge.get()).values[0].value).toBe(incrementValue);
    });

    it("decrement gauge value", async () => {
      // Arrange
      metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);
      const initialValue = 5;
      const decrementValue = 2;
      const expectedValue = initialValue - decrementValue;

      // Act
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, initialValue);
      metricService.decrementGauge(ExampleMetrics.ExampleMetrics, {}, decrementValue);

      // Assert
      expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(expectedValue);
    });

    it("set gauge to specific value", async () => {
      // Arrange
      metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);
      const targetValue = 12;

      // Act
      metricService.setGauge(ExampleMetrics.ExampleMetrics, {}, targetValue);

      // Assert
      expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(targetValue);
    });

    it("set gauge value when labels are omitted", async () => {
      // Arrange
      metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);
      const targetValue = 9;

      // Act
      metricService.setGauge(ExampleMetrics.ExampleMetrics, undefined, targetValue);

      // Assert
      expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(targetValue);
    });

    it("increment and decrement gauge using default step", async () => {
      // Arrange
      metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);
      const expectedValue = 1; // 2 increments - 1 decrement = 1

      // Act
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics);
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics);
      metricService.decrementGauge(ExampleMetrics.ExampleMetrics);

      // Assert
      expect(await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {})).toBe(expectedValue);
    });

    it("return undefined when setting non-existent gauge", async () => {
      // Arrange
      const targetValue = 7;

      // Act
      metricService.setGauge(ExampleMetrics.ExampleMetrics, {}, targetValue);

      // Assert
      const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
      expect(value).toBeUndefined();
    });

    it("return undefined when incrementing non-existent gauge", async () => {
      // Arrange
      const incrementValue = 4;

      // Act
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, incrementValue);

      // Assert
      const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
      expect(value).toBeUndefined();
    });

    it("return undefined when decrementing non-existent gauge", async () => {
      // Arrange
      const decrementValue = 2;

      // Act
      metricService.decrementGauge(ExampleMetrics.ExampleMetrics, {}, decrementValue);

      // Assert
      const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
      expect(value).toBeUndefined();
    });

    it("return gauge value after increment", async () => {
      // Arrange
      metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION);
      const incrementValue = 10;

      // Act
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, {}, incrementValue);

      // Assert
      const gaugeValue = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});
      expect(gaugeValue).toBe(incrementValue);
    });

    it("return undefined for non-existent gauge value", async () => {
      // Arrange
      // (no setup needed)

      // Act
      const value = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, {});

      // Assert
      expect(value).toBeUndefined();
    });

    it("aggregate gauge values across matching labels", async () => {
      // Arrange
      const labelNames = ["status", "region"];
      metricService.createGauge(ExampleMetrics.ExampleMetrics, TEST_GAUGE_DESCRIPTION, labelNames);
      const usOkValue = 2;
      const euOkValue = 3;
      const usFailValue = 5;
      const expectedOkAggregate = usOkValue + euOkValue;
      const expectedFailAggregate = usFailValue;

      // Act
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, { status: "ok", region: "us" }, usOkValue);
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, { status: "ok", region: "eu" }, euOkValue);
      metricService.incrementGauge(ExampleMetrics.ExampleMetrics, { status: "fail", region: "us" }, usFailValue);

      // Assert
      const aggregatedOk = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, { status: "ok" });
      const aggregatedFail = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, { status: "fail" });
      const missing = await metricService.getGaugeValue(ExampleMetrics.ExampleMetrics, { status: "missing" });

      expect(aggregatedOk).toBe(expectedOkAggregate);
      expect(aggregatedFail).toBe(expectedFailAggregate);
      expect(missing).toBeUndefined();
    });
  });

  describe("Histogram Operations", () => {
    it("create a histogram instance", async () => {
      // Arrange
      // (no setup needed)

      // Act
      const histogram = metricService.createHistogram(
        ExampleMetrics.ExampleMetrics,
        HISTOGRAM_BUCKETS,
        TEST_HISTOGRAM_DESCRIPTION,
      );

      // Assert
      expect(histogram).toBeInstanceOf(Histogram);
    });

    it("reuse existing histogram when creating duplicate", () => {
      // Arrange
      const histogram = metricService.createHistogram(
        ExampleMetrics.ExampleMetrics,
        HISTOGRAM_BUCKETS_SMALL,
        TEST_HISTOGRAM_DESCRIPTION,
        ["status"],
      );

      // Act
      const histogramAgain = metricService.createHistogram(
        ExampleMetrics.ExampleMetrics,
        HISTOGRAM_BUCKETS_SMALL,
        TEST_HISTOGRAM_DESCRIPTION,
        ["status"],
      );

      // Assert
      expect(histogramAgain).toBe(histogram);
    });

    it("add values to histogram and retrieve metrics", async () => {
      // Arrange
      metricService.createHistogram(ExampleMetrics.ExampleMetrics, HISTOGRAM_BUCKETS, TEST_HISTOGRAM_DESCRIPTION);
      const firstValue = 0.3;
      const secondValue = 1.5;

      // Act
      metricService.addValueToHistogram(ExampleMetrics.ExampleMetrics, firstValue);
      metricService.addValueToHistogram(ExampleMetrics.ExampleMetrics, secondValue);

      // Assert
      const histogramValues = await metricService.getHistogramMetricsValues(ExampleMetrics.ExampleMetrics);
      expect(histogramValues?.values.length).toBe(EXPECTED_HISTOGRAM_VALUES_LENGTH);
    });

    it("return undefined for non-existent histogram metrics", async () => {
      // Arrange
      // (no setup needed)

      // Act
      const values = await metricService.getHistogramMetricsValues(ExampleMetrics.ExampleMetrics);

      // Assert
      expect(values).toBeUndefined();
    });

    it("return undefined when adding value to non-existent histogram", async () => {
      // Arrange
      const testValue = 1.2;

      // Act
      metricService.addValueToHistogram(ExampleMetrics.ExampleMetrics, testValue);

      // Assert
      const values = await metricService.getHistogramMetricsValues(ExampleMetrics.ExampleMetrics);
      expect(values).toBeUndefined();
    });
  });

  describe("Registry Operations", () => {
    it("expose registry instance", () => {
      // Arrange
      // (no setup needed)

      // Act
      const registry = (metricService as SingletonMetricsService<ExampleMetrics>).getRegistry();

      // Assert
      expect(registry).toBeInstanceOf(Registry);
    });
  });
});
