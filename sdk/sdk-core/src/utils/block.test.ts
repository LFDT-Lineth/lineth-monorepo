import { parseBlockExtraData } from "./block";

import type { Hex } from "../types/misc";

describe("block", () => {
  describe("parseBlockExtraData", () => {
    it("should parse extra data with all zero values", () => {
      // version=0x00, fixedCost=0x00000000, variableCost=0x00000000, ethGasPrice=0x00000000
      const extraData = "0x00000000000000000000000000" as Hex;
      expect(parseBlockExtraData(extraData)).toEqual({
        version: 0,
        fixedCost: 0,
        variableCost: 0,
        ethGasPrice: 0,
      });
    });

    it("should parse extra data with version=1 and return costs in wei", () => {
      // version=0x01
      // fixedCost=0x00000064 (100 -> 100 * 1000 = 100_000 wei)
      // variableCost=0x000000c8 (200 -> 200 * 1000 = 200_000 wei)
      // ethGasPrice=0x0000012c (300 -> 300 * 1000 = 300_000 wei)
      const extraData = "0x0100000064000000c80000012c" as Hex;
      expect(parseBlockExtraData(extraData)).toEqual({
        version: 1,
        fixedCost: 100_000,
        variableCost: 200_000,
        ethGasPrice: 300_000,
      });
    });

    it("should return fixedCost, variableCost, and ethGasPrice in wei (raw value * 1000)", () => {
      // version=0x02, fixedCost=0x00000001 (1), variableCost=0x00000002 (2), ethGasPrice=0x00000003 (3)
      // Results are in wei: raw * 1000
      const extraData = "0x02000000010000000200000003" as Hex;
      const result = parseBlockExtraData(extraData);

      expect(result.version).toBe(2);
      expect(result.fixedCost).toBe(1_000);
      expect(result.variableCost).toBe(2_000);
      expect(result.ethGasPrice).toBe(3_000);
    });
  });
});
