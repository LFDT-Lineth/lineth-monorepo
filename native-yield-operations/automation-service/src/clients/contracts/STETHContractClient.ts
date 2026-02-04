import { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import { Address, getContract, GetContractReturnType, PublicClient, TransactionReceipt } from "viem";
import { ISTETH } from "../../core/clients/contracts/ISTETH.js";
import { STETHABI } from "../../core/abis/STETH.js";

/**
 * Client for interacting with STETH smart contracts.
 * Provides methods for reading STETH contract state information.
 */
export class STETHContractClient implements ISTETH {
  private readonly contract: GetContractReturnType<typeof STETHABI, PublicClient, Address>;

  /**
   * Creates a new STETHContractClient instance.
   *
   * @param {IBlockchainClient<PublicClient, TransactionReceipt>} contractClientLibrary - Blockchain client for reading contract data.
   * @param {Address} contractAddress - The address of the STETH contract.
   * @param {ILogger} logger - Logger instance for logging operations.
   */
  constructor(
    private readonly contractClientLibrary: IBlockchainClient<PublicClient, TransactionReceipt>,
    private readonly contractAddress: Address,
    private readonly logger: ILogger,
  ) {
    this.contract = getContract({
      abi: STETHABI,
      address: contractAddress,
      client: this.contractClientLibrary.getBlockchainClient(),
    });
  }

  /**
   * Gets the address of the STETH contract.
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
   * Gets the balance of the STETH contract.
   *
   * @returns {Promise<bigint>} The contract balance in wei.
   */
  async getBalance(): Promise<bigint> {
    return this.contractClientLibrary.getBalance(this.contractAddress);
  }

  /**
   * Gets the pooled ETH amount for a given shares amount, rounded up.
   * Reads the getPooledEthBySharesRoundUp function which returns the ETH equivalent of the shares.
   *
   * @param {bigint} sharesAmount - The shares amount to convert.
   * @returns {Promise<bigint | undefined>} The pooled ETH amount in wei, or undefined on error.
   */
  async getPooledEthBySharesRoundUp(sharesAmount: bigint): Promise<bigint | undefined> {
    try {
      const value = await this.contract.read.getPooledEthBySharesRoundUp([sharesAmount]);
      return value ?? 0n;
    } catch (error) {
      this.logger.error(`getPooledEthBySharesRoundUp failed, error=${error}`);
      return undefined;
    }
  }
}
