import { describe, afterEach, it, expect, beforeEach } from "@jest/globals";
import { toBeHex } from "ethers";
import { MockProxy, mock, mockClear } from "jest-mock-extended";

import { DEFAULT_GAS_ESTIMATION_PERCENTILE, DEFAULT_MAX_FEE_PER_GAS_CAP } from "../../../core/constants";
import { Direction } from "../../../core/enums/message";
import { DEFAULT_MAX_FEE_PER_GAS } from "../../../utils/testing/constants/common";
import { generateTransactionRequest } from "../../../utils/testing/helpers";
import { Provider } from "../../providers/provider";
import { GasProvider } from "../GasProvider";

const testFeeHistory = {
  baseFeePerGas: ["0x3da8e7618", "0x3e1ba3b1b", "0x3dfd72b90", "0x3d64eee76", "0x3d4da2da0", "0x3ccbcac6b"],
  gasUsedRatio: [0.5290747666666666, 0.49240453333333334, 0.4615576, 0.49407083333333335, 0.4669053],
  oldestBlock: "0xfab8ac",
  reward: [
    ["0xe4e1c0", "0xe4e1c0"],
    ["0xe4e1c0", "0xe4e1c0"],
    ["0xe4e1c0", "0xe4e1c0"],
    ["0xcf7867", "0xe4e1c0"],
    ["0x5f5e100", "0xe4e1c0"],
  ],
};

describe("GasProvider", () => {
  let providerMock: MockProxy<Provider>;
  let gasProvider: GasProvider;

  beforeEach(() => {
    providerMock = mock<Provider>();
    gasProvider = new GasProvider(providerMock, {
      enableLineaEstimateGas: true,
      direction: Direction.L1_TO_L2,
      enforceMaxGasFee: false,
      maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
      gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
    });
  });

  afterEach(() => {
    mockClear(providerMock);
    jest.clearAllMocks();
  });

  describe("getGasFees", () => {
    describe("L1 to L2", () => {
      it("should throw an error if transactionRequest param is undefined and enableLineaEstimateGas is enabled", async () => {
        await expect(gasProvider.getGasFees()).rejects.toThrow(
          "You need to provide transaction request as param to call the getGasFees function on Linea.",
        );
      });

      it("should use LineaGasProvider when enableLineaEstimateGas is enabled", async () => {
        jest.spyOn(providerMock, "send").mockResolvedValueOnce({
          gasLimit: "0x300000",
          baseFeePerGas: "0x7",
          priorityFeePerGas: toBeHex(DEFAULT_MAX_FEE_PER_GAS),
        });
        const gasFees = await gasProvider.getGasFees(generateTransactionRequest());

        const expectedBaseFee = (BigInt("0x7") * BigInt(1.35 * 100)) / 100n;
        const expectedPriorityFeePerGas = (DEFAULT_MAX_FEE_PER_GAS * BigInt(1.05 * 100)) / 100n;
        const expectedMaxFeePerGas = expectedBaseFee + expectedPriorityFeePerGas;

        expect(gasFees).toStrictEqual({
          gasLimit: 3145728n,
          maxFeePerGas: expectedMaxFeePerGas,
          maxPriorityFeePerGas: expectedPriorityFeePerGas,
        });
      });

      it("should use DefaultGasProvider when enableLineaEstimateGas is disabled", async () => {
        gasProvider = new GasProvider(providerMock, {
          enableLineaEstimateGas: false,
          direction: Direction.L1_TO_L2,
          enforceMaxGasFee: false,
          maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
          gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
        });

        jest.spyOn(providerMock, "getBlockNumber").mockResolvedValueOnce(1);
        const estimatedGasLimit = 50_000n;
        jest.spyOn(providerMock, "estimateGas").mockResolvedValueOnce(estimatedGasLimit);
        jest.spyOn(providerMock, "send").mockResolvedValueOnce(testFeeHistory);

        const gasFees = await gasProvider.getGasFees();

        expect(gasFees).toStrictEqual({
          gasLimit: estimatedGasLimit,
          maxFeePerGas: 32671357073n,
          maxPriorityFeePerGas: 31719355n,
        });
      });
    });

    describe("L2 to L1", () => {
      it("should use DefaultGasProvider", async () => {
        gasProvider = new GasProvider(providerMock, {
          enableLineaEstimateGas: false,
          direction: Direction.L2_TO_L1,
          enforceMaxGasFee: false,
          maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
          gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
        });

        jest.spyOn(providerMock, "getBlockNumber").mockResolvedValueOnce(1);
        jest.spyOn(providerMock, "send").mockResolvedValueOnce(testFeeHistory);

        const gasFees = await gasProvider.getGasFees();

        expect(gasFees).toStrictEqual({
          maxFeePerGas: 32671357073n,
          maxPriorityFeePerGas: 31719355n,
        });
      });
    });
  });

  describe("getMaxFeePerGas", () => {
    it("should use LineaGasProvider if direction == L1_TO_L2", () => {
      expect(gasProvider.getMaxFeePerGas()).toStrictEqual(DEFAULT_MAX_FEE_PER_GAS);
    });

    it("should use DefaultGasProvider if direction == L2_TO_L1", () => {
      gasProvider = new GasProvider(providerMock, {
        enableLineaEstimateGas: false,
        direction: Direction.L2_TO_L1,
        enforceMaxGasFee: false,
        maxFeePerGasCap: DEFAULT_MAX_FEE_PER_GAS_CAP,
        gasEstimationPercentile: DEFAULT_GAS_ESTIMATION_PERCENTILE,
      });
      expect(gasProvider.getMaxFeePerGas()).toStrictEqual(DEFAULT_MAX_FEE_PER_GAS);
    });
  });
});
