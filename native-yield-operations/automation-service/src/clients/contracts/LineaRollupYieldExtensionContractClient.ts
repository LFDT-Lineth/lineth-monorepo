import { IBlockchainClient, ILogger } from "@consensys/linea-shared-utils";
import {
  Address,
  encodeFunctionData,
  getContract,
  GetContractReturnType,
  PublicClient,
  TransactionReceipt,
} from "viem";
import { LineaRollupYieldExtensionABI } from "../../core/abis/LineaRollupYieldExtension.js";
import { ILineaRollupYieldExtension } from "../../core/clients/contracts/ILineaRollupYieldExtension.js";

/**
 * Client for interacting with LineaRollupYieldExtension smart contracts.
 * Provides methods for transferring funds for native yield operations on the Linea rollup.
 */
export class LineaRollupYieldExtensionContractClient implements ILineaRollupYieldExtension<TransactionReceipt> {
  private readonly contract: GetContractReturnType<typeof LineaRollupYieldExtensionABI, PublicClient, Address>;

  /**
   * Creates a new LineaRollupYieldExtensionContractClient instance.
   *
   * @param {ILogger} logger - Logger instance for logging operations.
   * @param {IBlockchainClient<PublicClient, TransactionReceipt>} contractClientLibrary - Blockchain client for sending transactions.
   * @param {Address} contractAddress - The address of the LineaRollupYieldExtension contract.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly contractClientLibrary: IBlockchainClient<PublicClient, TransactionReceipt>,
    private readonly contractAddress: Address,
  ) {
    this.contract = getContract({
      abi: LineaRollupYieldExtensionABI,
      address: contractAddress,
      client: contractClientLibrary.getBlockchainClient(),
    });
  }

  /**
   * Gets the address of the LineaRollupYieldExtension contract.
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
   * Gets the balance of the LineaRollupYieldExtension contract.
   *
   * @returns {Promise<bigint>} The contract balance in wei.
   */
  async getBalance(): Promise<bigint> {
    return this.contractClientLibrary.getBalance(this.contractAddress);
  }

  /**
   * Transfers funds for native yield operations on the Linea rollup.
   * Encodes the function call and sends a signed transaction via the blockchain client.
   *
   * @param {bigint} amount - The amount to transfer in wei.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   */
  async transferFundsForNativeYield(amount: bigint): Promise<TransactionReceipt> {
    this.logger.debug(`transferFundsForNativeYield started, amount=${amount.toString()}`);
    const calldata = encodeFunctionData({
      abi: this.contract.abi,
      functionName: "transferFundsForNativeYield",
      args: [amount],
    });

    const txReceipt = await this.contractClientLibrary.sendSignedTransaction(
      this.contractAddress,
      calldata,
      undefined,
      LineaRollupYieldExtensionABI,
    );
    this.logger.info(
      `transferFundsForNativeYield succeeded, amount=${amount.toString()}, txHash=${txReceipt.transactionHash}`,
    );
    return txReceipt;
  }
}
