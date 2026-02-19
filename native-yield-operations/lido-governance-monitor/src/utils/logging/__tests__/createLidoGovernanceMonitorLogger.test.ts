import { describe, it, expect } from "@jest/globals";

import { createLidoGovernanceMonitorLogger } from "../createLidoGovernanceMonitorLogger.js";
import { LidoGovernanceMonitorLogger } from "../LidoGovernanceMonitorLogger.js";

describe("createLidoGovernanceMonitorLogger", () => {
  it("returns a LidoGovernanceMonitorLogger instance", () => {
    const logger = createLidoGovernanceMonitorLogger("TestComponent");

    expect(logger).toBeInstanceOf(LidoGovernanceMonitorLogger);
  });

  it("sets the logger name correctly", () => {
    const logger = createLidoGovernanceMonitorLogger("ProposalFetcher");

    expect(logger.name).toBe("ProposalFetcher");
  });

  it("creates loggers with distinct names", () => {
    const logger1 = createLidoGovernanceMonitorLogger("ServiceA");
    const logger2 = createLidoGovernanceMonitorLogger("ServiceB");

    expect(logger1.name).toBe("ServiceA");
    expect(logger2.name).toBe("ServiceB");
  });
});
