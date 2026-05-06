import { describe, afterEach, it, beforeEach } from "@jest/globals";
import { FeeData } from "ethers";

import { Provider } from "..";
import { DEFAULT_MAX_FEE_PER_GAS } from "../../../utils/testing/constants/common";

describe("Provider", () => {
  let provider: Provider;

  beforeEach(() => {
    provider = new Provider();
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("getFees", () => {
    it("should throw an error when getFeeData function does not return `maxPriorityFeePerGas` or `maxFeePerGas` values", async () => {
      jest.spyOn(provider, "getFeeData").mockResolvedValue({
        maxPriorityFeePerGas: null,
        maxFeePerGas: null,
        gasPrice: 10n,
      } as FeeData);

      await expect(provider.getFees()).rejects.toThrow("Error getting fee data");
    });

    it("should return `maxPriorityFeePerGas` and `maxFeePerGas` values", async () => {
      jest.spyOn(provider, "getFeeData").mockResolvedValue({
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasPrice: 10n,
      } as FeeData);

      expect(await provider.getFees()).toStrictEqual({
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });
    });
  });
});
