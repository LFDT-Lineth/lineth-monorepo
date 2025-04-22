import { Counter, Gauge, Registry } from "prom-client";

export enum LineaPostmanMetrics {
  Messages = "linea_postman_messages",
}

export interface IMetricsService {
  getRegistry(): Registry;
  createCounter(name: LineaPostmanMetrics, help: string, labelNames?: string[]): Counter<string>;
  createGauge(name: LineaPostmanMetrics, help: string, labelNames?: string[]): Gauge<string>;
  incrementCounter(name: LineaPostmanMetrics, labels?: Record<string, string>, value?: number): void;
  incrementGauge(name: LineaPostmanMetrics, labels?: Record<string, string>, value?: number): void;
  decrementGauge(name: LineaPostmanMetrics, labels?: Record<string, string>, value?: number): void;
  getGaugeValue(name: LineaPostmanMetrics, labels: Record<string, string>): Promise<number | undefined>;
  getCounterValue(name: LineaPostmanMetrics, labels: Record<string, string>): Promise<number | undefined>;
}
