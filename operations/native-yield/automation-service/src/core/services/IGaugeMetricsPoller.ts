/**
 * Interface for polling and updating gauge metrics from various data sources.
 * Provides a single method to refresh all gauge metrics that need periodic updates.
 */
export interface IGaugeMetricsPoller {
  /**
   * Polls data sources and updates gauge metrics.
   * This method should be idempotent and safe to call repeatedly.
   *
   * @returns {Promise<void>} A promise that resolves when gauge metrics are updated.
   */
  poll(): Promise<void>;
}
