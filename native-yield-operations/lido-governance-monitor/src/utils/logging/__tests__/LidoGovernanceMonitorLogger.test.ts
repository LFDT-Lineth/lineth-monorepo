import { ILogger } from "@consensys/linea-shared-utils";
import { jest, describe, it, expect, beforeEach } from "@jest/globals";

import { Severity } from "../ILidoGovernanceMonitorLogger.js";
import { LidoGovernanceMonitorLogger } from "../LidoGovernanceMonitorLogger.js";

describe("LidoGovernanceMonitorLogger", () => {
  let mockBaseLogger: jest.Mocked<ILogger>;
  let logger: LidoGovernanceMonitorLogger;

  beforeEach(() => {
    mockBaseLogger = {
      name: "TestLogger",
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    };
    logger = new LidoGovernanceMonitorLogger(mockBaseLogger);
  });

  describe("critical", () => {
    it("calls base logger error with severity CRITICAL", () => {
      logger.critical("Database connection failed", { host: "localhost" });

      expect(mockBaseLogger.error).toHaveBeenCalledWith("Database connection failed", {
        severity: Severity.CRITICAL,
        host: "localhost",
      });
    });

    it("handles call without metadata", () => {
      logger.critical("Network timeout");

      expect(mockBaseLogger.error).toHaveBeenCalledWith("Network timeout", {
        severity: Severity.CRITICAL,
      });
    });
  });

  describe("error", () => {
    it("calls base logger error with severity ERROR", () => {
      logger.error("Validation failed", { field: "riskScore" });

      expect(mockBaseLogger.error).toHaveBeenCalledWith("Validation failed", {
        severity: Severity.ERROR,
        field: "riskScore",
      });
    });

    it("handles call without metadata", () => {
      logger.error("Parse error");

      expect(mockBaseLogger.error).toHaveBeenCalledWith("Parse error", {
        severity: Severity.ERROR,
      });
    });
  });

  describe("warn", () => {
    it("calls base logger warn with severity WARN", () => {
      logger.warn("Audit channel failed", { channel: "audit" });

      expect(mockBaseLogger.warn).toHaveBeenCalledWith("Audit channel failed", {
        severity: Severity.WARN,
        channel: "audit",
      });
    });

    it("handles call without metadata", () => {
      logger.warn("Retrying operation");

      expect(mockBaseLogger.warn).toHaveBeenCalledWith("Retrying operation", {
        severity: Severity.WARN,
      });
    });
  });

  describe("delegated methods", () => {
    it("delegates info to base logger", () => {
      logger.info("Starting process", { step: 1 });

      expect(mockBaseLogger.info).toHaveBeenCalledWith("Starting process", { step: 1 });
    });

    it("delegates debug to base logger", () => {
      logger.debug("Debug details", { data: "test" });

      expect(mockBaseLogger.debug).toHaveBeenCalledWith("Debug details", { data: "test" });
    });
  });

  describe("name property", () => {
    it("returns base logger name", () => {
      expect(logger.name).toBe("TestLogger");
    });
  });
});
