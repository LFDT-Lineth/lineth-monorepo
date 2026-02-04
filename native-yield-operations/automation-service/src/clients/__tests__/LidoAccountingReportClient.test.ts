import { describe, it, expect, beforeEach, jest } from "@jest/globals";
import type { IRetryService } from "@consensys/linea-shared-utils";
import type { Address, Hex, TransactionReceipt } from "viem";

import { createLoggerMock } from "../../__tests__/helpers/index.js";
import type { ILazyOracle } from "../../core/clients/contracts/ILazyOracle.js";
import { LidoAccountingReportClient } from "../LidoAccountingReportClient.js";

jest.mock("@lidofinance/lsv-cli/dist/utils/report/report-proof.js", () => ({
  getReportProofByVault: jest.fn(),
}));

import { getReportProofByVault } from "@lidofinance/lsv-cli/dist/utils/report/report-proof.js";

const mockedGetReportProofByVault = getReportProofByVault as jest.MockedFunction<typeof getReportProofByVault>;

type LazyOracleMock = jest.Mocked<ILazyOracle<TransactionReceipt>>;

// Test constants
const VAULT_ADDRESS = "0x1111111111111111111111111111111111111111" as Address;
const IPFS_GATEWAY_URL = "https://ipfs.example.com";
const REPORT_CID = "bafkreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const TREE_ROOT = "0x1234" as Hex;
const PROOF_HEX = "0xabc" as Hex;

// Test amounts in wei (as strings from API)
const TOTAL_VALUE_WEI = "1000";
const FEE_WEI = "200";
const LIABILITY_SHARES = "300";
const MAX_LIABILITY_SHARES = "400";
const SLASHING_RESERVE = "500";

// Expected bigint conversions
const TOTAL_VALUE_BIGINT = 1000n;
const FEE_BIGINT = 200n;
const LIABILITY_SHARES_BIGINT = 300n;
const MAX_LIABILITY_SHARES_BIGINT = 400n;
const SLASHING_RESERVE_BIGINT = 500n;

describe("LidoAccountingReportClient", () => {
  let logger: ReturnType<typeof createLoggerMock>;
  let retryService: jest.Mocked<IRetryService>;
  let retryMock: jest.Mock;
  let lazyOracle: LazyOracleMock;
  let client: LidoAccountingReportClient;

  const createMockLatestReportData = () => ({
    timestamp: 1n,
    refSlot: 2n,
    treeRoot: TREE_ROOT,
    reportCid: REPORT_CID,
  });

  const createMockReportProof = () => ({
    data: {
      totalValueWei: TOTAL_VALUE_WEI,
      fee: FEE_WEI,
      liabilityShares: LIABILITY_SHARES,
      maxLiabilityShares: MAX_LIABILITY_SHARES,
      slashingReserve: SLASHING_RESERVE,
    },
    proof: [PROOF_HEX],
  });

  const createRetryServiceMock = (): jest.Mocked<IRetryService> => {
    const mock = jest.fn((fn: () => Promise<unknown>) => fn());
    retryMock = mock as unknown as jest.Mock;
    return {
      retry: mock as unknown as IRetryService["retry"],
    } as unknown as jest.Mocked<IRetryService>;
  };

  const createLazyOracleMock = (): LazyOracleMock =>
    ({
      latestReportData: jest.fn(),
      updateVaultData: jest.fn(),
    }) as unknown as LazyOracleMock;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = createLoggerMock();
    retryService = createRetryServiceMock();
    lazyOracle = createLazyOracleMock();

    client = new LidoAccountingReportClient(logger, retryService, lazyOracle, IPFS_GATEWAY_URL);
  });

  describe("getLatestSubmitVaultReportParams", () => {
    it("fetches report from IPFS and converts string values to bigint", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);

      // Act
      const params = await client.getLatestSubmitVaultReportParams(VAULT_ADDRESS);

      // Assert
      expect(params.vault).toBe(VAULT_ADDRESS);
      expect(params.totalValue).toBe(TOTAL_VALUE_BIGINT);
      expect(params.cumulativeLidoFees).toBe(FEE_BIGINT);
      expect(params.liabilityShares).toBe(LIABILITY_SHARES_BIGINT);
      expect(params.maxLiabilityShares).toBe(MAX_LIABILITY_SHARES_BIGINT);
      expect(params.slashingReserve).toBe(SLASHING_RESERVE_BIGINT);
      expect(params.proof).toEqual([PROOF_HEX]);
    });

    it("retrieves report proof from IPFS gateway with retry", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);

      // Act
      await client.getLatestSubmitVaultReportParams(VAULT_ADDRESS);

      // Assert
      expect(retryService.retry).toHaveBeenCalledTimes(1);
      expect(mockedGetReportProofByVault).toHaveBeenCalledWith({
        vault: VAULT_ADDRESS,
        cid: REPORT_CID,
        gateway: IPFS_GATEWAY_URL,
      });
    });

    it("logs report parameters after successful fetch", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);

      // Act
      await client.getLatestSubmitVaultReportParams(VAULT_ADDRESS);

      // Assert
      expect(logger.info).toHaveBeenCalledWith(
        expect.stringContaining(`getLatestSubmitVaultReportParams for vault=${VAULT_ADDRESS}`),
      );
    });

    it("returns fresh parameters on each call to getLatestSubmitVaultReportParams", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);

      // Act
      await client.getLatestSubmitVaultReportParams(VAULT_ADDRESS);
      await client.getLatestSubmitVaultReportParams(VAULT_ADDRESS);

      // Assert
      expect(lazyOracle.latestReportData).toHaveBeenCalledTimes(2);
      expect(mockedGetReportProofByVault).toHaveBeenCalledTimes(2);
    });
  });

  describe("submitLatestVaultReport", () => {
    it("submits vault report using cached parameters", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);

      const params = await client.getLatestSubmitVaultReportParams(VAULT_ADDRESS);
      lazyOracle.latestReportData.mockClear();
      retryMock.mockClear();

      // Act
      await client.submitLatestVaultReport(VAULT_ADDRESS);

      // Assert
      expect(lazyOracle.updateVaultData).toHaveBeenCalledWith(params);
      expect(lazyOracle.latestReportData).not.toHaveBeenCalled();
    });

    it("fetches report parameters when cache is empty", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);
      lazyOracle.updateVaultData.mockResolvedValue(undefined as unknown as TransactionReceipt);

      // Act
      await client.submitLatestVaultReport(VAULT_ADDRESS);

      // Assert
      expect(lazyOracle.latestReportData).toHaveBeenCalledTimes(1);
      expect(mockedGetReportProofByVault).toHaveBeenCalledTimes(1);
    });

    it("submits all required vault data fields to lazy oracle", async () => {
      // Arrange
      const mockReportData = createMockLatestReportData();
      const mockReportProof = createMockReportProof();
      lazyOracle.latestReportData.mockResolvedValue(mockReportData);
      mockedGetReportProofByVault.mockResolvedValue(mockReportProof as any);
      lazyOracle.updateVaultData.mockResolvedValue(undefined as unknown as TransactionReceipt);

      // Act
      await client.submitLatestVaultReport(VAULT_ADDRESS);

      // Assert
      expect(lazyOracle.updateVaultData).toHaveBeenCalledWith(
        expect.objectContaining({
          vault: VAULT_ADDRESS,
          totalValue: TOTAL_VALUE_BIGINT,
          cumulativeLidoFees: FEE_BIGINT,
          liabilityShares: LIABILITY_SHARES_BIGINT,
          maxLiabilityShares: MAX_LIABILITY_SHARES_BIGINT,
          slashingReserve: SLASHING_RESERVE_BIGINT,
          proof: [PROOF_HEX],
        }),
      );
    });
  });
});
