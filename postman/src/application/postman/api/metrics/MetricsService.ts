import { Counter, Gauge, MetricObjectWithValues, MetricValue, Registry } from "prom-client";
import { IMetricsService, LineaPostmanMetrics } from "../../../../core/metrics/IMetricsService";

/**
 * MetricsService class that implements the IMetricsService interface.
 * This class provides methods to create and manage Prometheus metrics.
 */
export abstract class MetricsService implements IMetricsService {
  private readonly registry: Registry;
  private readonly counters: Map<LineaPostmanMetrics, Counter<string>>;
  private readonly gauges: Map<LineaPostmanMetrics, Gauge<string>>;

  constructor() {
    this.registry = new Registry();
    this.registry.setDefaultLabels({ app: "postman" });

    this.counters = new Map();
    this.gauges = new Map();
  }

  /**
   * Returns the registry
   * @returns {Registry} The registry instance
   */
  public getRegistry(): Registry {
    return this.registry;
  }

  /**
   * Creates counter metric
   */
  public createCounter(name: LineaPostmanMetrics, help: string, labelNames: string[] = []): Counter<string> {
    if (!this.counters.has(name)) {
      this.counters.set(
        name,
        new Counter({
          name,
          help,
          labelNames,
          registers: [this.registry],
        }),
      );
    }
    return this.counters.get(name) as Counter<string>;
  }

  /**
   * Get counter metric value
   * @param name - Name of the metric
   * @param labels - Labels for the metric
   * @returns Value of the counter metric
   */
  public async getCounterValue(name: LineaPostmanMetrics, labels: Record<string, string>): Promise<number | undefined> {
    const counter = this.counters.get(name);
    if (counter === undefined) {
      return undefined;
    }

    const metricData = await counter.get();
    const metricValueWithMatchingLabels = this.findMetricValueWithExactMatchingLabels(metricData, labels);
    return metricValueWithMatchingLabels?.value;
  }

  /**
   * Creates gauge metric
   * @param name - Name of the metric
   * @param help - Help text for the metric
   * @param labelNames - Array of label names for the metric
   * @returns Gauge metric
   */
  public createGauge(name: LineaPostmanMetrics, help: string, labelNames: string[] = []): Gauge<string> {
    if (!this.gauges.has(name)) {
      this.gauges.set(
        name,
        new Gauge({
          name,
          help,
          labelNames,
          registers: [this.registry],
        }),
      );
    }
    return this.gauges.get(name) as Gauge<string>;
  }

  /**
   * Get gauge metric value
   * @param name - Name of the metric
   * @param labels - Labels for the metric
   * @returns Value of the gauge metric
   */
  public async getGaugeValue(name: LineaPostmanMetrics, labels: Record<string, string>): Promise<number | undefined> {
    const gauge = this.gauges.get(name);

    if (gauge === undefined) {
      return undefined;
    }

    const metricData = await gauge.get();
    const metricValueWithMatchingLabels = this.findMetricValueWithExactMatchingLabels(metricData, labels);
    return metricValueWithMatchingLabels?.value;
  }

  /**
   * Increments a counter metric
   * @param name - Name of the metric
   * @param labels - Labels for the metric
   * @param value - Value to increment by (default is 1)
   * @returns void
   */
  public incrementCounter(name: LineaPostmanMetrics, labels: Record<string, string> = {}, value?: number): void {
    const counter = this.counters.get(name);
    if (counter !== undefined) {
      counter.inc(labels, value);
    }
  }

  /**
   * Increment a gauge metric value
   * @param name - Name of the metric
   * @param labels - Labels for the metric
   * @param value - Value to increment by (default is 1)
   * @returns void
   */
  public incrementGauge(name: LineaPostmanMetrics, labels: Record<string, string> = {}, value?: number): void {
    const gauge = this.gauges.get(name);
    if (gauge !== undefined) {
      gauge.inc(labels, value);
    }
  }

  /**
   * Decrement a gauge metric value
   * @param name - Name of the metric
   * @param value - Value to decrement by (default is 1)
   * @param labels - Labels for the metric
   * @returns void
   */
  public decrementGauge(name: LineaPostmanMetrics, labels: Record<string, string> = {}, value?: number): void {
    const gauge = this.gauges.get(name);
    if (gauge !== undefined) {
      gauge.dec(labels, value);
    }
  }

  private findMetricValueWithExactMatchingLabels(
    metricData: MetricObjectWithValues<MetricValue<string>>,
    labels: Record<string, string>,
  ): MetricValue<string> | undefined {
    return metricData.values.find((value) => Object.entries(labels).every(([key, val]) => value.labels[key] === val));
  }
}
