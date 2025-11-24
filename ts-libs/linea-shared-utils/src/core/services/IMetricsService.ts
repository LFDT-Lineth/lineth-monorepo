import { Counter, Gauge, Histogram, MetricObjectWithValues, MetricValueWithName, Registry } from "prom-client";

export interface IMetricsService<TMetricName extends string = string> {
  getRegistry(): Registry;
  createCounter(name: TMetricName, help: string, labelNames?: string[]): Counter<string>;
  createGauge(name: TMetricName, help: string, labelNames?: string[]): Gauge<string>;
  incrementCounter(name: TMetricName, labels?: Record<string, string>, value?: number): void;
  setGauge(name: TMetricName, labels?: Record<string, string>, value?: number): void;
  incrementGauge(name: TMetricName, labels?: Record<string, string>, value?: number): void;
  decrementGauge(name: TMetricName, labels?: Record<string, string>, value?: number): void;
  getGaugeValue(name: TMetricName, labels: Record<string, string>): Promise<number | undefined>;
  getCounterValue(name: TMetricName, labels: Record<string, string>): Promise<number | undefined>;
  createHistogram(name: TMetricName, buckets: number[], help: string, labelNames?: string[]): Histogram<string>;
  addValueToHistogram(name: TMetricName, value: number, labels?: Record<string, string>): void;
  getHistogramMetricsValues(
    name: TMetricName,
  ): Promise<MetricObjectWithValues<MetricValueWithName<string>> | undefined>;
}
