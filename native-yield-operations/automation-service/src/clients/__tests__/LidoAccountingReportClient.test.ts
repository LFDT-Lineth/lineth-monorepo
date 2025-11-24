import { jest } from "@jest/globals";
import type { ILogger, IRetryService } from "@consensys/linea-shared-utils";
import type { ILazyOracle, UpdateVaultDataParams } from "../../core/clients/contracts/ILazyOracle.js";
import type { Address, Hex, TransactionReceipt } from "viem";
import { LidoAccountingReportClient } from "../LidoAccountingReportClient.js";

jest.mock("@lidofinance/lsv-cli/dist/utils/report/report-proof.js", () => ({
  getReportProofByVault: jest.fn(),
}));

import { getReportProofByVault } from "@lidofinance/lsv-cli/dist/utils/report/report-proof.js";

const mockedGetReportProofByVault = getReportProofByVault as jest.MockedFunction<typeof getReportProofByVault>;

type LazyOracleMock = jest.Mocked<ILazyOracle<TransactionReceipt>>;

describe("LidoAccountingReportClient", () => {
  const vault = "0x1111111111111111111111111111111111111111" as Address;
  const ipfsGatewayUrl = "https://ipfs.example.com";

  let logger: jest.Mocked<ILogger>;
  let retryService: jest.Mocked<IRetryService>;
  let retryMock: jest.Mock;
  let lazyOracle: LazyOracleMock;
  let client: LidoAccountingReportClient;

  const reportData = {
    timestamp: 1n,
    refSlot: 2n,
    treeRoot: "0x1234" as Hex,
    reportCid: "bafkreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  };

  const reportProof = {
    data: {
      totalValueWei: "1000",
      fee: "200",
      liabilityShares: "300",
      maxLiabilityShares: "400",
      slashingReserve: "500",
    },
    proof: ["0xabc"] as Hex[],
  } as const;

  const createLoggerMock = (): jest.Mocked<ILogger> =>
    ({
      name: "test-logger",
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    }) as unknown as jest.Mocked<ILogger>;

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
    lazyOracle.latestReportData.mockResolvedValue(reportData);
    mockedGetReportProofByVault.mockResolvedValue(reportProof as any);

    client = new LidoAccountingReportClient(logger, retryService, lazyOracle, ipfsGatewayUrl);
  });

  const expectReportParams = (params: UpdateVaultDataParams) => {
    expect(params).toEqual({
      vault,
      totalValue: 1000n,
      cumulativeLidoFees: 200n,
      liabilityShares: 300n,
      maxLiabilityShares: 400n,
      slashingReserve: 500n,
      proof: reportProof.proof,
    });
  };

  it("fetches the latest report, converts values to bigint, logs, and caches the params", async () => {
    const params = await client.getLatestSubmitVaultReportParams(vault);

    expectReportParams(params);
    expect(retryService.retry).toHaveBeenCalledTimes(1);
    expect(mockedGetReportProofByVault).toHaveBeenCalledWith({
      vault,
      cid: reportData.reportCid,
      gateway: ipfsGatewayUrl,
    });
    expect(logger.info).toHaveBeenCalledWith(
      expect.stringContaining(`getLatestSubmitVaultReportParams for vault=${vault}`),
    );
  });

  it("submits the latest vault report using cached params", async () => {
    const params = await client.getLatestSubmitVaultReportParams(vault);
    lazyOracle.latestReportData.mockClear();
    retryMock.mockClear();

    await client.submitLatestVaultReport(vault);

    expect(lazyOracle.updateVaultData).toHaveBeenCalledWith(params);
    expect(lazyOracle.latestReportData).not.toHaveBeenCalled();
  });

  it("submits the latest vault report fetching params when cache is empty", async () => {
    lazyOracle.updateVaultData.mockResolvedValue(undefined as unknown as TransactionReceipt);

    await client.submitLatestVaultReport(vault);

    expect(lazyOracle.latestReportData).toHaveBeenCalledTimes(1);
    expect(mockedGetReportProofByVault).toHaveBeenCalledTimes(1);
    expect(lazyOracle.updateVaultData).toHaveBeenCalledWith(
      expect.objectContaining({
        vault,
        totalValue: 1000n,
        cumulativeLidoFees: 200n,
        liabilityShares: 300n,
        maxLiabilityShares: 400n,
        slashingReserve: 500n,
        proof: reportProof.proof,
      }),
    );
  });
});
