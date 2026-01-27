import { describe, afterEach, jest, it, expect, beforeEach } from "@jest/globals";
import { MockProxy, mock, mockClear } from "jest-mock-extended";

import { DEFAULT_GAS_ESTIMATION_PERCENTILE } from "../../../core/constants";
import { makeBaseError } from "../../../core/errors";
import { Provider } from "../../providers/provider";
import { DefaultGasProvider } from "../DefaultGasProvider";

const MAX_FEE_PER_GAS = 100_000_000n;

describe("DefaultGasProvider", () => {
  let providerMock: MockProxy<Provider>;
  let eip1559GasProvider: DefaultGasProvider;
  beforeEach(() => {
    providerMock = mock<Provider>();
    eip1559GasProvider = new DefaultGasProvider(providerMock, {
      maxFeePerGasCap: MAX_FEE_PER_GAS,
      gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
      enforceMaxGasFee: false,
    });
  });

  afterEach(() => {
    mockClear(providerMock);
  });

  describe("getGasFees", () => {
    it("should return fee from cache if currentBlockNumber == cacheIsValidForBlockNumber", async () => {
      jest.spyOn(providerMock, "getBlockNumber").mockResolvedValueOnce(0);
      const fees = await eip1559GasProvider.getGasFees();

      expect(fees).toStrictEqual({
        maxFeePerGas: MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: MAX_FEE_PER_GAS,
      });
    });

    it("should throw an error 'FeeEstimationError' if maxPriorityFee is greater than maxFeePerGas", async () => {
      jest.spyOn(providerMock, "getBlockNumber").mockResolvedValueOnce(1);
      const sendSpy = jest.spyOn(providerMock, "send").mockResolvedValueOnce({
        baseFeePerGas: ["0x3da8e7618", "0x3e1ba3b1b", "0x3dfd72b90", "0x3d64eee76", "0x3d4da2da0", "0x3ccbcac6b"],
        gasUsedRatio: [0.5290747666666666, 0.49240453333333334, 0.4615576, 0.49407083333333335, 0.4669053],
        oldestBlock: "0xfab8ac",
        reward: [
          ["0x59682f00", "0x59682f00"],
          ["0x59682f00", "0x59682f00"],
          ["0x3b9aca00", "0x59682f00"],
          ["0x510b0870", "0x59682f00"],
          ["0x3b9aca00", "0x59682f00"],
        ],
      });
      await expect(eip1559GasProvider.getGasFees()).rejects.toThrow(
        makeBaseError(`Estimated miner tip of ${1271935510} exceeds configured max fee per gas of ${MAX_FEE_PER_GAS}!`),
      );
      expect(sendSpy).toHaveBeenCalledTimes(1);
    });

    it("should return maxFeePerGas and maxPriorityFeePerGas", async () => {
      jest.spyOn(providerMock, "getBlockNumber").mockResolvedValueOnce(1);
      const sendSpy = jest.spyOn(providerMock, "send").mockResolvedValueOnce({
        baseFeePerGas: ["0x3da8e", "0x3e1ba", "0x3dfd7", "0x3d64e", "0x3d4da", "0x3ccbc"],
        gasUsedRatio: [0.5290747666666666, 0.49240453333333334, 0.4615576, 0.49407083333333335, 0.4669053],
        oldestBlock: "0xfab8ac",
        reward: [
          ["0xe4e1c0", "0xe4e1c0"],
          ["0xe4e1c0", "0xe4e1c0"],
          ["0xe4e1c0", "0xe4e1c0"],
          ["0xcf7867", "0xe4e1c0"],
          ["0x5f5e100", "0xe4e1c0"],
        ],
      });

      const fees = await eip1559GasProvider.getGasFees();

      expect(fees).toStrictEqual({
        maxFeePerGas: 32_217_395n,
        maxPriorityFeePerGas: 31_719_355n,
      });

      expect(sendSpy).toHaveBeenCalledTimes(1);
    });

    it("should return maxFeePerGas from config when maxFeePerGas and maxPriorityFeePerGas === 0", async () => {
      jest.spyOn(providerMock, "getBlockNumber").mockResolvedValueOnce(1);

      const sendSpy = jest.spyOn(providerMock, "send").mockResolvedValueOnce({
        baseFeePerGas: ["0x0", "0x0", "0x0", "0x0", "0x0", "0x0"],
        gasUsedRatio: [0, 0, 0, 0, 0],
        oldestBlock: "0xfab8ac",
        reward: [
          ["0x0", "0x0"],
          ["0x0", "0x0"],
          ["0x0", "0x0"],
          ["0x0", "0x0"],
          ["0x0", "0x0"],
        ],
      });

      const fees = await eip1559GasProvider.getGasFees();

      expect(fees).toStrictEqual({
        maxFeePerGas: MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: MAX_FEE_PER_GAS,
      });

      expect(sendSpy).toHaveBeenCalledTimes(1);
    });
  });
});
