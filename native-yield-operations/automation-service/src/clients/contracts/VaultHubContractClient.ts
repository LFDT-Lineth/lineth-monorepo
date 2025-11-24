import { IBlockchainClient } from "@consensys/linea-shared-utils";
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
   */
  constructor(
    private readonly contractClientLibrary: IBlockchainClient<PublicClient, TransactionReceipt>,
    private readonly contractAddress: Address,
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

    const etherWithdrawn =
      logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase())?.args.etherWithdrawn ?? 0n;
    return etherWithdrawn;
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

    const transferred =
      logs.find((log) => log.address.toLowerCase() === this.contractAddress.toLowerCase())?.args.transferred ?? 0n;
    return transferred;
  }
}
