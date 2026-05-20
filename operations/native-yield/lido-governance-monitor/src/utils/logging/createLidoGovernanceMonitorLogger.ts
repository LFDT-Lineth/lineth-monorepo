import { WinstonLogger } from "@consensys/linea-shared-utils";

import { ILidoGovernanceMonitorLogger } from "./ILidoGovernanceMonitorLogger.js";
import { LidoGovernanceMonitorLogger } from "./LidoGovernanceMonitorLogger.js";

/**
 * Factory function to create a severity-aware logger for the Lido Governance Monitor.
 *
 * @param name - Component name (e.g., "ProposalFetcher", "ClaudeAIClient")
 * @param logLevel - Log level (default: from LOG_LEVEL env or "info")
 * @returns ILidoGovernanceMonitorLogger instance
 */
export function createLidoGovernanceMonitorLogger(name: string, logLevel?: string): ILidoGovernanceMonitorLogger {
  const level = logLevel ?? process.env.LOG_LEVEL ?? "info";
  const baseLogger = new WinstonLogger(name, { level });
  return new LidoGovernanceMonitorLogger(baseLogger);
}
