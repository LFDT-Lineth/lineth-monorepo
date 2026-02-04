import { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import { Address, getContract, GetContractReturnType, PublicClient, TransactionReceipt } from "viem";
import { IStakingVault } from "../../core/clients/contracts/IStakingVault.js";
import { StakingVaultABI } from "../../core/abis/StakingVault.js";

/**
 * Client for interacting with StakingVault smart contracts.
 * Provides methods for reading staking vault state information.
 */
export class StakingVaultContractClient implements IStakingVault {
  private static readonly clientCache = new Map<Address, StakingVaultContractClient>();
  private static blockchainClient: IBlockchainClient<PublicClient, TransactionReceipt> | undefined;
  private static logger: ILogger | undefined;

  private readonly contract: GetContractReturnType<typeof StakingVaultABI, PublicClient, Address>;

  /**
   * Initializes the static blockchain client and logger for all StakingVaultContractClient instances.
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
   * Creates a new StakingVaultContractClient instance.
   * Requires blockchainClient and logger to be initialized via initialize() before use.
   *
   * @param {Address} contractAddress - The address of the StakingVault contract.
   */
  constructor(private readonly contractAddress: Address) {
    if (!StakingVaultContractClient.blockchainClient) {
      throw new Error(
        "StakingVaultContractClient: blockchainClient must be initialized via StakingVaultContractClient.initialize() before use",
      );
    }
    if (!StakingVaultContractClient.logger) {
      throw new Error(
        "StakingVaultContractClient: logger must be initialized via StakingVaultContractClient.initialize() before use",
      );
    }
    this.contract = getContract({
      abi: StakingVaultABI,
      address: contractAddress,
      client: StakingVaultContractClient.blockchainClient.getBlockchainClient(),
    });
  }

  /**
   * Gets or creates a StakingVaultContractClient instance for the given staking vault address.
   * Uses a static cache to reuse instances for the same staking vault address.
   * Requires blockchainClient to be initialized via initialize() before first use.
   *
   * @param {Address} contractAddress - The address of the StakingVault contract.
   * @returns {StakingVaultContractClient} The cached or newly created StakingVaultContractClient instance.
   * @throws {Error} If blockchainClient has not been initialized.
   */
  static getOrCreate(contractAddress: Address): StakingVaultContractClient {
    if (!this.blockchainClient) {
      throw new Error(
        "StakingVaultContractClient: blockchainClient must be initialized via StakingVaultContractClient.initialize() before use",
      );
    }
    if (!this.logger) {
      throw new Error(
        "StakingVaultContractClient: logger must be initialized via StakingVaultContractClient.initialize() before use",
      );
    }

    const cached = this.clientCache.get(contractAddress);
    if (cached) {
      return cached;
    }

    const client = new StakingVaultContractClient(contractAddress);
    this.clientCache.set(contractAddress, client);
    return client;
  }

  /**
   * Gets the address of the StakingVault contract.
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
   * Gets the balance of the StakingVault contract.
   *
   * @returns {Promise<bigint>} The contract balance in wei.
   */
  async getBalance(): Promise<bigint> {
    if (!StakingVaultContractClient.blockchainClient) {
      throw new Error(
        "StakingVaultContractClient: blockchainClient must be initialized via StakingVaultContractClient.initialize() before use",
      );
    }
    return StakingVaultContractClient.blockchainClient.getBalance(this.contractAddress);
  }

  /**
   * Checks if beacon chain deposits are paused for the staking vault.
   *
   * @returns {Promise<boolean>} True if beacon chain deposits are paused, false otherwise.
   */
  async beaconChainDepositsPaused(): Promise<boolean> {
    return this.contract.read.beaconChainDepositsPaused();
  }
}
