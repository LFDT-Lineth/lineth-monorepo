import { describe, expect, it } from "@jest/globals";

import { computeEffectiveRisk } from "../effectiveRisk.js";

describe("computeEffectiveRisk", () => {
  it("rounds riskScore multiplied by confidence over 100", () => {
    const riskScore = 75;
    const confidence = 85;

    const result = computeEffectiveRisk(riskScore, confidence);

    expect(result).toBe(64);
  });

  it("returns the provided effectiveRisk when present", () => {
    const riskScore = 75;
    const confidence = 85;
    const effectiveRisk = 91;

    const result = computeEffectiveRisk(riskScore, confidence, effectiveRisk);

    expect(result).toBe(91);
  });
});
