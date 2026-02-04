import { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import { Address, getContract, GetContractReturnType, parseEventLogs, PublicClient, TransactionReceipt } from "viem";
import { IDashboard } from "../../core/clients/contracts/IDashboard.js";
import { DashboardABI } from "../../core/abis/Dashboard.js";

/**
 * Client for interacting with Dashboard smart contracts.
 * Provides methods for extracting payment information from transaction receipts by decoding contract events.
 */
export class DashboardContractClient implements IDashboard<TransactionReceipt> {
  private static readonly clientCache = new Map<Address, DashboardContractClient>();
  private static blockchainClient: IBlockchainClient<PublicClient, TransactionReceipt> | undefined;
  private static logger: ILogger | undefined;

  private readonly contract: GetContractReturnType<typeof DashboardABI, PublicClient, Address>;
  private readonly logger: ILogger;

  /**
   * Initializes the static blockchain client and logger for all DashboardContractClient instances.
   * Should be called once during application startup.
   *
   * @param {IBlockchainClient<PublicClient, TransactionReceipt>} blockchainClient - Blockchain client for reading contract data.
   * @param {ILogger} logger - Logger instance for logging operations.
   */
  static initialize(blockchainClient: IBlockchainClient<PublicClient, TransactionReceipt>, logger: ILogger): void {
    this.blockchainClient = blockchainClient;
    this.logger = logger;
  }

  /**
   * Creates a new DashboardContractClient instance.
   * Requires blockchainClient and logger to be initialized via initialize() before use.
   *
   * @param {Address} contractAddress - The address of the Dashboard contract.
   */
  constructor(private readonly contractAddress: Address) {
    if (!DashboardContractClient.blockchainClient) {
      throw new Error(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    }
    if (!DashboardContractClient.logger) {
      throw new Error(
        "DashboardContractClient: logger must be initialized via DashboardContractClient.initialize() before use",
      );
    }
    this.logger = DashboardContractClient.logger;
    this.contract = getContract({
      abi: DashboardABI,
      address: contractAddress,
      client: DashboardContractClient.blockchainClient.getBlockchainClient(),
    });
  }

  /**
   * Gets or creates a DashboardContractClient instance for the given dashboard address.
   * Uses a static cache to reuse instances for the same dashboard address.
   * Requires blockchainClient to be initialized via initialize() before first use.
   *
   * @param {Address} contractAddress - The address of the Dashboard contract.
   * @returns {DashboardContractClient} The cached or newly created DashboardContractClient instance.
   * @throws {Error} If blockchainClient has not been initialized.
   */
  static getOrCreate(contractAddress: Address): DashboardContractClient {
    if (!this.blockchainClient) {
      throw new Error(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    }
    if (!this.logger) {
      throw new Error(
        "DashboardContractClient: logger must be initialized via DashboardContractClient.initialize() before use",
      );
    }

    const cached = this.clientCache.get(contractAddress);
    if (cached) {
      return cached;
    }

    const client = new DashboardContractClient(contractAddress);
    this.clientCache.set(contractAddress, client);
    return client;
  }

  /**
   * Gets the address of the Dashboard contract.
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
   * Gets the balance of the Dashboard contract.
   *
   * @returns {Promise<bigint>} The contract balance in wei.
   */
  async getBalance(): Promise<bigint> {
    if (!DashboardContractClient.blockchainClient) {
      throw new Error(
        "DashboardContractClient: blockchainClient must be initialized via DashboardContractClient.initialize() before use",
      );
    }
    return DashboardContractClient.blockchainClient.getBalance(this.contractAddress);
  }

  /**
   * Extracts the node operator fee amount from a transaction receipt by decoding FeeDisbursed events.
   * Only decodes logs emitted by this contract. Skips unrelated logs (from the same contract or different ABIs).
   * If event not found, returns 0n.
   *
   * @param {TransactionReceipt} txReceipt - The transaction receipt to search for FeeDisbursed events.
   * @returns {bigint} The fee amount from the FeeDisbursed event, or 0n if the event is not found.
   */
  getNodeOperatorFeesPaidFromTxReceipt(txReceipt: TransactionReceipt): bigint {
    const logs = parseEventLogs({
      abi: this.contract.abi,
      eventName: "FeeDisbursed",
      logs: txReceipt.logs,
    });

    const event = logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase());
    if (!event) {
      this.logger.warn("getNodeOperatorFeesPaidFromTxReceipt - FeeDisbursed event not found in receipt");
      return 0n;
    }

    return event.args.fee ?? 0n;
  }

  /**
   * Gets the withdrawable value from the Dashboard contract.
   *
   * @returns {Promise<bigint>} The withdrawable value in wei.
   */
  async withdrawableValue(): Promise<bigint> {
    return this.contract.read.withdrawableValue();
  }

  /**
   * Gets the total value from the Dashboard contract.
   *
   * @returns {Promise<bigint>} The total value in wei.
   */
  async totalValue(): Promise<bigint> {
    return this.contract.read.totalValue();
  }

  /**
   * Gets the liability shares from the Dashboard contract.
   *
   * @returns {Promise<bigint>} The liability shares.
   */
  async liabilityShares(): Promise<bigint> {
    return this.contract.read.liabilityShares();
  }
}
