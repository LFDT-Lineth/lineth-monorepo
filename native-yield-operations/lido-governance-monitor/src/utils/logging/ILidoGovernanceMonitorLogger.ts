import { ILogger } from "@consensys/linea-shared-utils";

export enum Severity {
  CRITICAL = "CRITICAL",
  ERROR = "ERROR",
  WARN = "WARN",
}

/**
 * Extended logger interface that adds severity-classified logging methods.
 * - critical(): External dependency failures (HTTP errors, DB errors, API failures)
 * - error(): Processing failures not caused by external deps (validation errors)
 * - warn(): Non-blocking issues worth noting
 *
 * All severity methods auto-inject { severity: Severity } into log metadata.
 */
export interface ILidoGovernanceMonitorLogger extends ILogger {
  /**
   * Log CRITICAL severity - external dependency failures.
   * Use when: HTTP errors, DB errors, Slack webhook fails, API request errors.
   */
  critical(message: string, meta?: Record<string, unknown>): void;
}
