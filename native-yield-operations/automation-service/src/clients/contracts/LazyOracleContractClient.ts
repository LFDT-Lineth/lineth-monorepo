import { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import {
  Address,
  encodeFunctionData,
  getContract,
  GetContractReturnType,
  InvalidInputRpcError,
  PublicClient,
  TransactionReceipt,
  WatchContractEventReturnType,
} from "viem";
import { LazyOracleABI } from "../../core/abis/LazyOracle.js";
import {
  ILazyOracle,
  UpdateVaultDataParams,
  LazyOracleReportData,
  WaitForVaultReportDataEventResult,
  VaultReportResult,
} from "../../core/clients/contracts/ILazyOracle.js";
import { OperationTrigger } from "../../core/metrics/LineaNativeYieldAutomationServiceMetrics.js";

/**
 * Client for interacting with LazyOracle smart contracts.
 * Provides methods for reading report data, updating vault data, simulating transactions,
 * and waiting for VaultsReportDataUpdated events with timeout handling.
 */
export class LazyOracleContractClient implements ILazyOracle<TransactionReceipt> {
  private readonly contract: GetContractReturnType<typeof LazyOracleABI, PublicClient, Address>;
  /**
   * Creates a new LazyOracleContractClient instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {IBlockchainClient<PublicClient, TransactionReceipt>} contractClientLibrary - Blockchain client for sending transactions.
   * @param {Address} contractAddress - The address of the LazyOracle contract.
   * @param {number} pollIntervalMs - Polling interval in milliseconds for event watching.
   * @param {number} eventWatchTimeoutMs - Timeout in milliseconds for waiting for events.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly contractClientLibrary: IBlockchainClient<PublicClient, TransactionReceipt>,
    private readonly contractAddress: Address,
    private readonly pollIntervalMs: number,
    private readonly eventWatchTimeoutMs: number,
  ) {
    this.contract = getContract({
      abi: LazyOracleABI,
      address: contractAddress,
      client: contractClientLibrary.getBlockchainClient(),
    });
  }

  /**
   * Gets the address of the LazyOracle contract.
   *
   * @returns {Address} The contract address.
   */
  getAddress(): Address {
    return this.contractAddress;
  }

  /**
   * Gets the viem contract instance.
   *
   * @returns {GetContractReturnType} The contract instance.
   */
  getContract(): GetContractReturnType {
    return this.contract;
  }

  /**
   * Retrieves the latest report data from the LazyOracle contract.
   *
   * @returns {Promise<LazyOracleReportData>} The latest report data containing timestamp, refSlot, treeRoot, and reportCid.
   */
  async latestReportData(): Promise<LazyOracleReportData> {
    const [timestamp, refSlot, treeRoot, reportCid] = await this.contract.read.latestReportData();
    const returnVal = {
      timestamp,
      refSlot,
      treeRoot,
      reportCid,
    };
    this.logger.debug("latestReportData", { returnVal });
    return returnVal;
  }

  /**
   * Updates vault data in the LazyOracle contract by submitting a transaction.
   * Encodes the function call and sends a signed transaction via the blockchain client.
   *
   * @param {UpdateVaultDataParams} params - The vault data parameters including vault address, totalValue, cumulativeLidoFees, liabilityShares, maxLiabilityShares, slashingReserve, and proof.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async updateVaultData(params: UpdateVaultDataParams): Promise<TransactionReceipt> {
    const { vault, totalValue, cumulativeLidoFees, liabilityShares, maxLiabilityShares, slashingReserve, proof } =
      params;
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "updateVaultData",
      args: [vault, totalValue, cumulativeLidoFees, liabilityShares, maxLiabilityShares, slashingReserve, proof],
    });
    this.logger.debug(`updateVaultData started`, { params });
    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(this.contractAddress, calldata);
    this.logger.info(`updateVaultData succeeded, txHash=${txReceipt.transactionHash}`, { params });
    return txReceipt;
  }

  /**
   * Waits for a VaultsReportDataUpdated event from the LazyOracle contract.
   * Creates placeholder Promise resolve fn. Creates Promise that we will return.
   * Sets placeholder resolve fn, to resolve fn here - decouples Promise creation from resolve fn creation.
   * Creates placeholders for unwatch fns. Starts timeout and event watching.
   * Filters out removed logs (could be due to reorg) and logs that don't fit our interface.
   * On success, cleans up and returns. Tolerates errors - we don't want to interrupt L1MessageService<->YieldProvider rebalancing.
   *
   * @returns {Promise<WaitForVaultReportDataEventResult>} The event result containing the operation trigger and report data, or TIMEOUT if the event is not detected within the timeout period.
   */
  async waitForVaultsReportDataUpdatedEvent(): Promise<WaitForVaultReportDataEventResult> {
    // Create placeholder Promise resolve fn
    let resolvePromise: (value: WaitForVaultReportDataEventResult) => void;
    // Create Promise that we will return
    // Set placeholder resolve fn, to resolve fn here - decouple Promise creation from resolve fn creation
    const waitForEvent = new Promise<WaitForVaultReportDataEventResult>((resolve) => {
      resolvePromise = resolve;
    });

    // Create placeholders for unwatch fns
    let unwatchEvent: WatchContractEventReturnType | undefined;
    let unwatchTimeout: (() => void) | undefined;

    const cleanup = () => {
      if (unwatchTimeout) {
        unwatchTimeout();
        unwatchTimeout = undefined;
      }
      if (unwatchEvent) {
        unwatchEvent();
        unwatchEvent = undefined;
      }
    };

    // Start timeout
    this.logger.info(`waitForVaultsReportDataUpdatedEvent started with timeout=${this.eventWatchTimeoutMs}ms`);
    const timeoutId = setTimeout(() => {
      cleanup();
      this.logger.info(`waitForVaultsReportDataUpdatedEvent timed out after timeout=${this.eventWatchTimeoutMs}ms`);
      resolvePromise({ result: OperationTrigger.TIMEOUT });
    }, this.eventWatchTimeoutMs);
    unwatchTimeout = () => clearTimeout(timeoutId);

    // Start event watching
    unwatchEvent = this.contractClientLibrary.getBlockchainClient().watchContractEvent({
      address: this.contractAddress,
      abi: this.contract.abi,
      eventName: "VaultsReportDataUpdated",
      pollingInterval: this.pollIntervalMs,
      onLogs: (logs) => {
        // Filter out removed logs - could be due to reorg
        const nonRemovedLogs = logs.filter((log) => !log.removed);
        if (nonRemovedLogs.length === 0) {
          this.logger.warn("waitForVaultsReportDataUpdatedEvent: Dropped VaultsReportDataUpdated event");
          return;
        }

        if (nonRemovedLogs.length !== logs.length) {
          this.logger.debug("waitForVaultsReportDataUpdatedEvent: Ignored removed reorg logs", { logs });
        }

        // Filter out logs that don't fit our interface
        const firstEvent = nonRemovedLogs[0];
        if (
          firstEvent.args?.timestamp === undefined ||
          firstEvent.args?.refSlot === undefined ||
          firstEvent.args?.root === undefined ||
          firstEvent.args?.cid === undefined
        ) {
          return;
        }

        // Success -> cleanup and return
        cleanup();
        const result: VaultReportResult = {
          result: OperationTrigger.VAULTS_REPORT_DATA_UPDATED_EVENT,
          txHash: firstEvent.transactionHash,
          report: {
            timestamp: firstEvent.args?.timestamp,
            refSlot: firstEvent.args?.refSlot,
            treeRoot: firstEvent.args?.root,
            reportCid: firstEvent.args?.cid,
          },
        };
        this.logger.info("waitForVaultsReportDataUpdatedEvent detected", {
          result,
        });
        resolvePromise(result);
      },
      onError: (error) => {
        // This means a filter has expired and Viem will handle renewing it - https://github.com/wevm/viem/blob/003b231361f223487aa3e6a67a1e5258e8fe758b/src/actions/public/watchContractEvent.ts#L260-L265
        // A filter is a RPC-provider managed subscription for blockchain change notifications - https://chainstack.readme.io/reference/ethereum-filters-rpc-methods
        if (error instanceof InvalidInputRpcError) {
          this.logger.warn("waitForVaultsReportDataUpdatedEvent: Filter expired, will be recreated by Viem framework", {
            error,
          });
          return;
        }

        // Tolerate errors, we don't want to interrupt L1MessageService<->YieldProvider rebalancing
        this.logger.error("waitForVaultsReportDataUpdatedEvent error", { error });
      },
    });

    return await waitForEvent;
  }
}
