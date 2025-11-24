import { Address, TransactionReceipt } from "viem";
import { ILidoAccountingReportClient } from "../core/clients/ILidoAccountingReportClient.js";
import { ILazyOracle, UpdateVaultDataParams } from "../core/clients/contracts/ILazyOracle.js";
import { getReportProofByVault } from "@lidofinance/lsv-cli/dist/utils/report/report-proof.js";
import { ILogger, IRetryService, bigintReplacer } from "@consensys/linea-shared-utils";

/**
 * Client for submitting Lido accounting reports to the LazyOracle contract.
 * Retrieves report data from IPFS, caches vault report parameters, and provides methods
 * for submitting vault accounting reports.
 */
export class LidoAccountingReportClient implements ILidoAccountingReportClient {
  private vaultReportByAddress = new Map<Address, UpdateVaultDataParams>();

  /**
   * Creates a new LidoAccountingReportClient instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {IRetryService} retryService - Service for retrying failed operations.
   * @param {ILazyOracle<TransactionReceipt>} lazyOracleContractClient - Client for interacting with LazyOracle contracts.
   * @param {string} ipfsGatewayUrl - IPFS gateway URL for retrieving report data.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly retryService: IRetryService,
    private readonly lazyOracleContractClient: ILazyOracle<TransactionReceipt>,
    private readonly ipfsGatewayUrl: string,
  ) {}

  /**
   * Retrieves the latest vault report parameters for submission.
   * Fetches the latest report CID from the LazyOracle, retrieves the report proof from IPFS,
   * constructs the vault data parameters, and caches them for future use.
   *
   * @param {Address} vault - The vault address to get report parameters for.
   * @returns {Promise<UpdateVaultDataParams>} The vault data parameters including totalValue, cumulativeLidoFees, liabilityShares, maxLiabilityShares, slashingReserve, and proof.
   */
  async getLatestSubmitVaultReportParams(vault: Address): Promise<UpdateVaultDataParams> {
    const latestReportData = await this.lazyOracleContractClient.latestReportData();
    const reportProof = await this.retryService.retry(() =>
      getReportProofByVault({
        vault,
        cid: latestReportData.reportCid,
        gateway: this.ipfsGatewayUrl,
      }),
    );

    const params: UpdateVaultDataParams = {
      vault,
      totalValue: BigInt(reportProof.data.totalValueWei),
      cumulativeLidoFees: BigInt(reportProof.data.fee),
      liabilityShares: BigInt(reportProof.data.liabilityShares),
      maxLiabilityShares: BigInt(reportProof.data.maxLiabilityShares),
      slashingReserve: BigInt(reportProof.data.slashingReserve),
      proof: reportProof.proof,
    };

    this.vaultReportByAddress.set(vault, params);

    this.logger.info(
      `getLatestSubmitVaultReportParams for vault=${vault} latestSubmitVaultReportParams=${JSON.stringify(params, bigintReplacer, 2)}`,
    );
    return params;
  }

  /**
   * Submits the latest vault report to the LazyOracle contract.
   * Uses latest known result of this.getLatestSubmitVaultReportParams().
   *
   * @param {Address} vault - The vault address to submit the report for.
   * @returns {Promise<void>} A promise that resolves when the vault report is submitted.
   */
  async submitLatestVaultReport(vault: Address): Promise<void> {
    const params = await this._getLatestSubmitVaultReportParams(vault);
    await this.lazyOracleContractClient.updateVaultData(params);
  }

  /**
   * Gets the latest vault report parameters, using cached values if available.
   * If no cached value exists for the vault, fetches fresh parameters using getLatestSubmitVaultReportParams.
   *
   * @param {Address} vault - The vault address to get report parameters for.
   * @returns {Promise<UpdateVaultDataParams>} The vault data parameters, either from cache or freshly fetched.
   */
  private async _getLatestSubmitVaultReportParams(vault: Address): Promise<UpdateVaultDataParams> {
    const cachedVaultReport = this.vaultReportByAddress.get(vault);
    if (cachedVaultReport === undefined) {
      return this.getLatestSubmitVaultReportParams(vault);
    } else {
      return cachedVaultReport;
    }
  }
}
