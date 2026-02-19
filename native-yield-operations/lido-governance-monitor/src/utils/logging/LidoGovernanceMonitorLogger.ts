import { ILogger } from "@consensys/linea-shared-utils";

import { ILidoGovernanceMonitorLogger, Severity } from "./ILidoGovernanceMonitorLogger.js";

/**
 * Logger wrapper that adds severity-classified logging methods.
 * Wraps an ILogger and auto-injects { severity: Severity } into metadata.
 */
export class LidoGovernanceMonitorLogger implements ILidoGovernanceMonitorLogger {
  constructor(private readonly baseLogger: ILogger) {}

  get name(): string {
    return this.baseLogger.name;
  }

  critical(message: string, meta?: Record<string, unknown>): void {
    this.baseLogger.error(message, { severity: Severity.CRITICAL, ...meta });
  }

  error(message: string, meta?: Record<string, unknown>): void {
    this.baseLogger.error(message, { severity: Severity.ERROR, ...meta });
  }

  warn(message: string, meta?: Record<string, unknown>): void {
    this.baseLogger.warn(message, { severity: Severity.WARN, ...meta });
  }

  info(message: string, ...params: unknown[]): void {
    this.baseLogger.info(message, ...params);
  }

  debug(message: string, ...params: unknown[]): void {
    this.baseLogger.debug(message, ...params);
  }
}
