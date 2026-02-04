import { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import { Address, getContract, GetContractReturnType, parseEventLogs, PublicClient, TransactionReceipt } from "viem";
import { IVaultHub } from "../../core/clients/contracts/IVaultHub.js";
import { VaultHubABI } from "../../core/abis/VaultHub.js";

/**
 * Client for interacting with VaultHub smart contracts.
 * Provides methods for extracting payment information from transaction receipts by decoding contract events.
 */
export class VaultHubContractClient implements IVaultHub<TransactionReceipt> {
  private readonly contract: GetContractReturnType<typeof VaultHubABI, PublicClient, Address>;

  /**
   * Creates a new VaultHubContractClient instance.
   *
   * @param {IBlockchainClient<PublicClient, TransactionReceipt>} contractClientLibrary - Blockchain client for reading contract data.
   * @param {Address} contractAddress - The address of the VaultHub contract.
   * @param {ILogger} logger - Logger instance for logging operations.
   */
  constructor(
    private readonly contractClientLibrary: IBlockchainClient<PublicClient, TransactionReceipt>,
    private readonly contractAddress: Address,
    private readonly logger: ILogger,
  ) {
    this.contract = getContract({
      abi: VaultHubABI,
      address: contractAddress,
      client: this.contractClientLibrary.getBlockchainClient(),
    });
  }

  /**
   * Gets the address of the VaultHub contract.
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
   * Gets the balance of the VaultHub contract.
   *
   * @returns {Promise<bigint>} The contract balance in wei.
   */
  async getBalance(): Promise<bigint> {
    return this.contractClientLibrary.getBalance(this.contractAddress);
  }

  /**
   * Extracts the liability payment amount from a transaction receipt by decoding VaultRebalanced events.
   * Only decodes logs emitted by this contract. Skips unrelated logs (from the same contract or different ABIs).
   * If event not found, returns 0n.
   *
   * @param {TransactionReceipt} txReceipt - The transaction receipt to search for VaultRebalanced events.
   * @returns {bigint} The etherWithdrawn amount from the VaultRebalanced event, or 0n if the event is not found.
   */
  getLiabilityPaymentFromTxReceipt(txReceipt: TransactionReceipt): bigint {
    const logs = parseEventLogs({
      abi: this.contract.abi,
      eventName: "VaultRebalanced",
      logs: txReceipt.logs,
    });

    const event = logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase());
    if (!event) {
      this.logger.warn("getLiabilityPaymentFromTxReceipt - VaultRebalanced event not found in receipt");
      return 0n;
    }

    return event.args.etherWithdrawn ?? 0n;
  }

  /**
   * Extracts the Lido fee payment amount from a transaction receipt by decoding LidoFeesSettled events.
   * Only decodes logs emitted by this contract. Skips unrelated logs (from the same contract or different ABIs).
   * If event not found, returns 0n.
   *
   * @param {TransactionReceipt} txReceipt - The transaction receipt to search for LidoFeesSettled events.
   * @returns {bigint} The transferred amount from the LidoFeesSettled event, or 0n if the event is not found.
   */
  getLidoFeePaymentFromTxReceipt(txReceipt: TransactionReceipt): bigint {
    const logs = parseEventLogs({
      abi: this.contract.abi,
      eventName: "LidoFeesSettled",
      logs: txReceipt.logs,
    });

    const event = logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase());
    if (!event) {
      this.logger.warn("getLidoFeePaymentFromTxReceipt - LidoFeesSettled event not found in receipt");
      return 0n;
    }

    return event.args.transferred ?? 0n;
  }

  /**
   * Gets the settleable Lido protocol fees value for a given vault.
   * Reads the settleableLidoFeesValue function which returns the amount of settleable Lido protocol fees.
   *
   * @param {Address} vault - The vault address to query.
   * @returns {Promise<bigint | undefined>} The settleable Lido protocol fees amount in wei, or undefined on error.
   */
  async settleableLidoFeesValue(vault: Address): Promise<bigint | undefined> {
    try {
      const value = await this.contract.read.settleableLidoFeesValue([vault]);
      return value ?? 0n;
    } catch (error) {
      this.logger.error(`settleableLidoFeesValue failed, error=${error}`);
      return undefined;
    }
  }

  /**
   * Gets the timestamp from the latest report for a given vault.
   * Reads the latestReport function which returns a Report struct containing totalValue, inOutDelta, and timestamp.
   *
   * @param {Address} vault - The vault address to query.
   * @returns {Promise<bigint>} The timestamp from the latest report, or 0n on error.
   */
  async getLatestVaultReportTimestamp(vault: Address): Promise<bigint> {
    try {
      const report = await this.contract.read.latestReport([vault]);
      return BigInt(report.timestamp ?? 0n);
    } catch (error) {
      this.logger.error(`getLatestVaultReportTimestamp failed, error=${error}`);
      return 0n;
    }
  }

  /**
   * Checks if the report for a given vault is fresh.
   * Reads the isReportFresh function which returns whether the latest report is considered fresh.
   *
   * @param {Address} vault - The vault address to query.
   * @returns {Promise<boolean>} True if the report is fresh, false otherwise or on error.
   */
  async isReportFresh(vault: Address): Promise<boolean> {
    try {
      const result = await this.contract.read.isReportFresh([vault]);
      return result ?? false;
    } catch (error) {
      this.logger.error(`isReportFresh failed, error=${error}`);
      return false;
    }
  }

  /**
   * Checks if a vault is connected to the VaultHub contract.
   * Reads the isVaultConnected function which returns whether the vault is connected.
   *
   * @param {Address} vault - The vault address to query.
   * @returns {Promise<boolean>} True if the vault is connected, false otherwise or on error.
   */
  async isVaultConnected(vault: Address): Promise<boolean> {
    try {
      const result = await this.contract.read.isVaultConnected([vault]);
      return result ?? false;
    } catch (error) {
      this.logger.error(`isVaultConnected failed, error=${error}`);
      return false;
    }
  }
}
