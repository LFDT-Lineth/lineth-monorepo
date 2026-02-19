import { describe, it, expect } from "@jest/globals";

import { Severity } from "../ILidoGovernanceMonitorLogger.js";

describe("ILidoGovernanceMonitorLogger", () => {
  it("Severity enum has correct values", () => {
    expect(Severity.CRITICAL).toBe("CRITICAL");
    expect(Severity.ERROR).toBe("ERROR");
    expect(Severity.WARN).toBe("WARN");
  });
});
