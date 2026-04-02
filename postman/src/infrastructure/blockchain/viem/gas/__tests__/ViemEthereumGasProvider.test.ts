import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { DefaultGasProviderConfig } from "../../../../../core/clients/blockchain/IGasProvider";
import { BaseError } from "../../../../../core/errors/BaseError";
import { ViemEthereumGasProvider } from "../ViemEthereumGasProvider";

import type { PublicClient } from "viem";

describe("ViemEthereumGasProvider", () => {
  let publicClient: ReturnType<typeof mock<PublicClient>>;
  let logger: ReturnType<typeof mock<ILogger>>;

  const baseConfig: DefaultGasProviderConfig = {
    maxFeePerGasCap: 100_000_000_000n,
    enforceMaxGasFee: false,
    gasEstimationPercentile: 50,
  };

  // baseFee=10gwei, rewards=[1gwei, 2gwei, 1gwei, 2gwei]
  // maxPriorityFee = (1+2+1+2)/4 = 1.5gwei
  // maxFee = 10gwei * 2 + 1.5gwei = 21.5gwei
  const defaultFeeHistory = {
    baseFeePerGas: [10_000_000_000n, 10_000_000_000n],
    reward: [[1_000_000_000n], [2_000_000_000n], [1_000_000_000n], [2_000_000_000n]],
    gasUsedRatio: [],
    oldestBlock: 99n,
  };

  beforeEach(() => {
    publicClient = mock<PublicClient>();
    logger = mock<ILogger>();
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("getGasFees", () => {
    it("calculates fees from feeHistory", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue(defaultFeeHistory as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      const fees = await provider.getGasFees();

      // maxPriorityFee = (1+2+1+2)/4 = 1.5gwei
      expect(fees.maxPriorityFeePerGas).toBe(1_500_000_000n);
      // maxFee = 10gwei * 2 + 1.5gwei = 21.5gwei
      expect(fees.maxFeePerGas).toBe(21_500_000_000n);
    });

    it("calls getFeeHistory with blockCount=4 and configured percentile", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue(defaultFeeHistory as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      await provider.getGasFees();

      expect(publicClient.getFeeHistory).toHaveBeenCalledWith({
        blockCount: 4,
        blockTag: "latest",
        rewardPercentiles: [50],
      });
    });

    it("caps maxFeePerGas at maxFeePerGasCap when it would exceed it", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue({
        baseFeePerGas: [60_000_000_000n, 60_000_000_000n],
        reward: [[1_000_000_000n], [1_000_000_000n], [1_000_000_000n], [1_000_000_000n]],
        gasUsedRatio: [],
        oldestBlock: 99n,
      } as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      const fees = await provider.getGasFees();

      // maxFee = 60gwei * 2 + 1gwei = 121gwei > cap(100gwei)
      expect(fees.maxFeePerGas).toBe(100_000_000_000n);
      expect(fees.maxPriorityFeePerGas).toBe(1_000_000_000n);
    });

    it("returns maxFeePerGasCap for both fees when enforceMaxGasFee is true", async () => {
      const capConfig: DefaultGasProviderConfig = { ...baseConfig, enforceMaxGasFee: true };
      const provider = new ViemEthereumGasProvider(publicClient, capConfig, logger);
      const fees = await provider.getGasFees();

      expect(fees.maxFeePerGas).toBe(100_000_000_000n);
      expect(fees.maxPriorityFeePerGas).toBe(100_000_000_000n);
      expect(publicClient.getBlockNumber).not.toHaveBeenCalled();
      expect(publicClient.getFeeHistory).not.toHaveBeenCalled();
    });

    it("throws when estimated priority fee exceeds maxFeePerGasCap", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue({
        baseFeePerGas: [1_000_000_000n, 1_000_000_000n],
        reward: [[200_000_000_000n], [200_000_000_000n], [200_000_000_000n], [200_000_000_000n]],
        gasUsedRatio: [],
        oldestBlock: 99n,
      } as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      await expect(provider.getGasFees()).rejects.toThrow(BaseError);
    });

    it("returns cached fees when block number has not advanced", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue(defaultFeeHistory as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      await provider.getGasFees();
      await provider.getGasFees();

      expect(publicClient.getFeeHistory).toHaveBeenCalledTimes(1);
    });

    it("refetches fees when block number advances", async () => {
      publicClient.getBlockNumber.mockResolvedValueOnce(100n).mockResolvedValueOnce(101n);
      publicClient.getFeeHistory.mockResolvedValue(defaultFeeHistory as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      await provider.getGasFees();
      await provider.getGasFees();

      expect(publicClient.getFeeHistory).toHaveBeenCalledTimes(2);
    });

    it("handles undefined reward in feeHistory by using empty array fallback", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue({
        baseFeePerGas: [10_000_000_000n, 10_000_000_000n],
        reward: undefined,
        gasUsedRatio: [],
        oldestBlock: 99n,
      } as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      await expect(provider.getGasFees()).rejects.toThrow();
    });

    it("falls back to maxFeePerGasCap for both fees when computed fees are zero", async () => {
      publicClient.getBlockNumber.mockResolvedValue(100n);
      publicClient.getFeeHistory.mockResolvedValue({
        baseFeePerGas: [0n, 0n],
        reward: [[0n], [0n], [0n], [0n]],
        gasUsedRatio: [],
        oldestBlock: 99n,
      } as never);

      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      const fees = await provider.getGasFees();

      expect(fees.maxFeePerGas).toBe(100_000_000_000n);
      expect(fees.maxPriorityFeePerGas).toBe(100_000_000_000n);
    });
  });

  describe("getMaxFeePerGas", () => {
    it("returns the cap from config", () => {
      const provider = new ViemEthereumGasProvider(publicClient, baseConfig, logger);
      expect(provider.getMaxFeePerGas()).toBe(100_000_000_000n);
    });
  });
});
