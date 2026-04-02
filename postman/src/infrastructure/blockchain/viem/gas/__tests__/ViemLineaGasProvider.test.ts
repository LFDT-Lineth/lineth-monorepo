import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { LineaGasProviderConfig } from "../../../../../core/clients/blockchain/IGasProvider";
import { BaseError } from "../../../../../core/errors/BaseError";
import { TransactionRequest } from "../../../../../core/types";
import { TEST_ADDRESS_1, TEST_ADDRESS_2 } from "../../../../../utils/testing/constants";
import { ViemLineaGasProvider } from "../ViemLineaGasProvider";

import type { PublicClient } from "viem";

const TEST_TX_REQUEST: TransactionRequest = {
  from: TEST_ADDRESS_1,
  to: TEST_ADDRESS_2,
  data: "0x",
  value: 0n,
};

describe("ViemLineaGasProvider", () => {
  let publicClient: ReturnType<typeof mock<PublicClient>>;
  let logger: ReturnType<typeof mock<ILogger>>;

  const baseConfig: LineaGasProviderConfig = {
    maxFeePerGasCap: 100_000_000_000n,
    enforceMaxGasFee: false,
  };

  beforeEach(() => {
    publicClient = mock<PublicClient>();
    logger = mock<ILogger>();
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("linea_estimateGas mode (default)", () => {
    it("maps linea_estimateGas hex response to LineaGasFees", async () => {
      jest.spyOn(publicClient, "request").mockResolvedValue({
        gasLimit: "0x5208", // 21000
        baseFeePerGas: "0x3B9ACA00", // 1_000_000_000
        priorityFeePerGas: "0x3B9ACA00", // 1_000_000_000
      } as never);

      const provider = new ViemLineaGasProvider(publicClient, baseConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      expect(fees.gasLimit).toBe(21000n);
      expect(fees.maxPriorityFeePerGas).toBe(1_000_000_000n);
      expect(fees.maxFeePerGas).toBe(2_000_000_000n); // base + priority
    });

    it("calls linea_estimateGas via viem/linea with correct params", async () => {
      jest.spyOn(publicClient, "request").mockResolvedValue({
        gasLimit: "0x5208",
        baseFeePerGas: "0x1",
        priorityFeePerGas: "0x1",
      } as never);

      const provider = new ViemLineaGasProvider(publicClient, baseConfig, logger);
      await provider.getGasFees(TEST_TX_REQUEST);

      expect(publicClient.request).toHaveBeenCalledWith(
        expect.objectContaining({
          method: "linea_estimateGas",
          params: [
            expect.objectContaining({
              from: TEST_TX_REQUEST.from,
              to: TEST_TX_REQUEST.to,
            }),
          ],
        }),
      );
    });

    it("uses linea_estimateGas when enableLineaEstimateGas is explicitly true", async () => {
      jest.spyOn(publicClient, "request").mockResolvedValue({
        gasLimit: "0x5208",
        baseFeePerGas: "0x1",
        priorityFeePerGas: "0x1",
      } as never);

      const config: LineaGasProviderConfig = { ...baseConfig, enableLineaEstimateGas: true };
      const provider = new ViemLineaGasProvider(publicClient, config, logger);
      await provider.getGasFees(TEST_TX_REQUEST);

      expect(publicClient.request).toHaveBeenCalledWith(expect.objectContaining({ method: "linea_estimateGas" }));
    });

    it("caps maxFeePerGas when enforceMaxGasFee is true and computed fee exceeds cap", async () => {
      jest.spyOn(publicClient, "request").mockResolvedValue({
        gasLimit: "0x5208",
        baseFeePerGas: "0x174876E800", // 100_000_000_000
        priorityFeePerGas: "0x174876E800", // 100_000_000_000
      } as never);

      const capConfig: LineaGasProviderConfig = { ...baseConfig, enforceMaxGasFee: true };
      const provider = new ViemLineaGasProvider(publicClient, capConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      expect(fees.maxFeePerGas).toBe(100_000_000_000n);
    });

    it("does not cap when enforceMaxGasFee is false", async () => {
      jest.spyOn(publicClient, "request").mockResolvedValue({
        gasLimit: "0x5208",
        baseFeePerGas: "0x174876E800", // 100_000_000_000
        priorityFeePerGas: "0x174876E800", // 100_000_000_000
      } as never);

      const provider = new ViemLineaGasProvider(publicClient, baseConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      expect(fees.maxFeePerGas).toBe(200_000_000_000n);
    });
  });

  describe("standard estimation mode (enableLineaEstimateGas: false)", () => {
    const standardConfig: LineaGasProviderConfig = {
      ...baseConfig,
      enableLineaEstimateGas: false,
      gasEstimationPercentile: 50,
    };

    it("uses eth_estimateGas and getFeeHistory instead of linea_estimateGas", async () => {
      const estimateGasSpy = jest.spyOn(publicClient, "estimateGas").mockResolvedValue(21000n);
      const feeHistorySpy = jest.spyOn(publicClient, "getFeeHistory").mockResolvedValue({
        baseFeePerGas: [1_000_000_000n, 1_000_000_000n, 1_000_000_000n, 1_000_000_000n, 1_200_000_000n],
        gasUsedRatio: [0.5, 0.5, 0.5, 0.5],
        oldestBlock: 100n,
        reward: [[500_000_000n], [600_000_000n], [500_000_000n], [400_000_000n]],
      });

      const provider = new ViemLineaGasProvider(publicClient, standardConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      expect(estimateGasSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          account: TEST_TX_REQUEST.from,
          to: TEST_TX_REQUEST.to,
        }),
      );
      expect(feeHistorySpy).toHaveBeenCalledWith(
        expect.objectContaining({
          blockCount: 4,
          blockTag: "latest",
          rewardPercentiles: [50],
        }),
      );

      expect(fees.gasLimit).toBe(21000n);
      // avg priority = (500M + 600M + 500M + 400M) / 4 = 500M
      expect(fees.maxPriorityFeePerGas).toBe(500_000_000n);
      // maxFee = latestBaseFee * 2 + priority = 1.2G * 2 + 0.5G = 2.9G
      expect(fees.maxFeePerGas).toBe(2_900_000_000n);
    });

    it("does not call linea_estimateGas", async () => {
      const requestSpy = jest.spyOn(publicClient, "request");
      jest.spyOn(publicClient, "estimateGas").mockResolvedValue(21000n);
      jest.spyOn(publicClient, "getFeeHistory").mockResolvedValue({
        baseFeePerGas: [1_000_000_000n, 1_000_000_000n],
        gasUsedRatio: [0.5],
        oldestBlock: 100n,
        reward: [[500_000_000n]],
      });

      const provider = new ViemLineaGasProvider(publicClient, standardConfig, logger);
      await provider.getGasFees(TEST_TX_REQUEST);

      const lineaCalls = requestSpy.mock.calls.filter((call) => (call[0] as any)?.method === "linea_estimateGas");
      expect(lineaCalls).toHaveLength(0);
    });

    it("caps maxFeePerGas when enforceMaxGasFee is true", async () => {
      jest.spyOn(publicClient, "estimateGas").mockResolvedValue(21000n);
      jest.spyOn(publicClient, "getFeeHistory").mockResolvedValue({
        baseFeePerGas: [100_000_000_000n, 100_000_000_000n],
        gasUsedRatio: [0.5],
        oldestBlock: 100n,
        reward: [[100_000_000_000n]],
      });

      const capConfig: LineaGasProviderConfig = { ...standardConfig, enforceMaxGasFee: true };
      const provider = new ViemLineaGasProvider(publicClient, capConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      // raw = 100G * 2 + 100G = 300G > cap of 100G
      expect(fees.maxFeePerGas).toBe(100_000_000_000n);
    });

    it("uses DEFAULT_GAS_ESTIMATION_PERCENTILE when gasEstimationPercentile is not set", async () => {
      const feeHistorySpy = jest.spyOn(publicClient, "getFeeHistory").mockResolvedValue({
        baseFeePerGas: [1_000_000_000n, 1_000_000_000n],
        gasUsedRatio: [0.5],
        oldestBlock: 100n,
        reward: [[500_000_000n]],
      });
      jest.spyOn(publicClient, "estimateGas").mockResolvedValue(21000n);

      const configWithoutPercentile: LineaGasProviderConfig = {
        ...baseConfig,
        enableLineaEstimateGas: false,
      };
      const provider = new ViemLineaGasProvider(publicClient, configWithoutPercentile, logger);
      await provider.getGasFees(TEST_TX_REQUEST);

      expect(feeHistorySpy).toHaveBeenCalledWith(
        expect.objectContaining({
          rewardPercentiles: [20], // DEFAULT_GAS_ESTIMATION_PERCENTILE
        }),
      );
    });

    it("handles empty rewards gracefully", async () => {
      jest.spyOn(publicClient, "estimateGas").mockResolvedValue(21000n);
      jest.spyOn(publicClient, "getFeeHistory").mockResolvedValue({
        baseFeePerGas: [1_000_000_000n, 1_000_000_000n],
        gasUsedRatio: [0.5],
        oldestBlock: 100n,
        reward: [],
      });

      const provider = new ViemLineaGasProvider(publicClient, standardConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      expect(fees.maxPriorityFeePerGas).toBe(0n);
      expect(fees.maxFeePerGas).toBe(2_000_000_000n); // latestBase * 2 + 0
    });

    it("handles undefined rewards from getFeeHistory", async () => {
      jest.spyOn(publicClient, "estimateGas").mockResolvedValue(21000n);
      jest.spyOn(publicClient, "getFeeHistory").mockResolvedValue({
        baseFeePerGas: [1_000_000_000n, 1_000_000_000n],
        gasUsedRatio: [0.5],
        oldestBlock: 100n,
      } as never);

      const provider = new ViemLineaGasProvider(publicClient, standardConfig, logger);
      const fees = await provider.getGasFees(TEST_TX_REQUEST);

      expect(fees.maxPriorityFeePerGas).toBe(0n);
      expect(fees.maxFeePerGas).toBe(2_000_000_000n);
    });
  });

  describe("shared behavior", () => {
    it("throws when from address is missing", async () => {
      const provider = new ViemLineaGasProvider(publicClient, baseConfig, logger);
      const noFromRequest: TransactionRequest = { to: TEST_TX_REQUEST.to, data: "0x", value: 0n };
      await expect(provider.getGasFees(noFromRequest)).rejects.toThrow(BaseError);
      await expect(provider.getGasFees(noFromRequest)).rejects.toThrow(
        "ViemLineaGasProvider: Transaction request must specify the 'from' address.",
      );
    });

    it("throws when from address is missing in standard mode", async () => {
      const config: LineaGasProviderConfig = { ...baseConfig, enableLineaEstimateGas: false };
      const provider = new ViemLineaGasProvider(publicClient, config, logger);
      const noFromRequest: TransactionRequest = { to: TEST_TX_REQUEST.to, data: "0x", value: 0n };
      await expect(provider.getGasFees(noFromRequest)).rejects.toThrow(
        "ViemLineaGasProvider: Transaction request must specify the 'from' address.",
      );
    });
  });

  describe("getMaxFeePerGas", () => {
    it("returns the cap from config", () => {
      const provider = new ViemLineaGasProvider(publicClient, baseConfig, logger);
      expect(provider.getMaxFeePerGas()).toBe(100_000_000_000n);
    });
  });
});
