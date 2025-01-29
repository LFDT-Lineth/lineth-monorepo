import {
  Overrides,
  ContractTransactionResponse,
  Signer,
  TransactionReceipt,
  TransactionResponse,
  ErrorDescription,
} from "ethers";
import { OnChainMessageStatus } from "../../core/enums/message";
import { Cache } from "../../utils/Cache";
import { ILineaRollupClient } from "../../core/clients/ethereum";
import { IL2MessageServiceClient, IL2MessageServiceLogClient } from "../../core/clients/linea";
import { MessageSent, Network } from "../../core/types";
import { FinalizationMessagingInfo, Proof } from "../../core/clients/ethereum/IMerkleTreeService";
import { makeBaseError } from "../../core/errors/utils";

export class L1ClaimingService {
  private cache: Cache;

  /**
   * Initializes a new instance of the L1ClaimingService.
   *
   * @param {ILineaRollupClient<Overrides, ContractTransactionResponse>} l1ContractClient - An instance of a class implementing the `ILineaRollupClient` interface, used to interact with the L1 contract.
   * @param {IL2MessageServiceClient} l2ContractClient - An instance of a class implementing the `IL2MessageServiceClient` interface, used to interact with the L2 contract.
   * @param {IL2MessageServiceLogClient} l2EventLogClient - An instance of a class implementing the `IL2MessageServiceLogClient` interface for fetching L2 event logs.
   * @param {Network} network - The network configuration.
   */
  constructor(
    public readonly l1ContractClient: ILineaRollupClient<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      ErrorDescription
    >,
    public readonly l2ContractClient: IL2MessageServiceClient<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      Signer,
      ErrorDescription
    >,
    public readonly l2EventLogClient: IL2MessageServiceLogClient,
    private readonly network: Network,
  ) {
    this.cache = new Cache();
  }

  /**
   * Retrieves finalization messaging information for a given transaction hash.
   *
   * @param {string} transactionHash - The hash of the transaction to retrieve information for.
   * @returns {Promise<FinalizationMessagingInfo>} Information about the finalization messaging.
   */
  public async getFinalizationMessagingInfo(transactionHash: string): Promise<FinalizationMessagingInfo> {
    return this.l1ContractClient.getFinalizationMessagingInfo(transactionHash);
  }

  /**
   * Fetches L2 message hashes within a specified block range.
   *
   * @param {number} fromBlock - The starting block number.
   * @param {number} toBlock - The ending block number.
   * @returns {Promise<string[]>} An array of message hashes found within the specified block range.
   */
  public async getL2MessageHashesInBlockRange(fromBlock: number, toBlock: number): Promise<string[]> {
    return this.l1ContractClient.getL2MessageHashesInBlockRange(fromBlock, toBlock);
  }

  /**
   * Retrieves message siblings for a given message hash.
   *
   * @param {string} messageHash - The hash of the message to retrieve siblings for.
   * @param {string[]} messageHashes - An array of message hashes to consider.
   * @param {number} treeDepth - The depth of the message tree.
   * @returns {string[]} An array of message siblings.
   */
  public getMessageSiblings(messageHash: string, messageHashes: string[], treeDepth: number): string[] {
    return this.l1ContractClient.getMessageSiblings(messageHash, messageHashes, treeDepth);
  }

  /**
   * Determines if a message was sent after a specified migration block number.
   *
   * @param {string} messageHash - The hash of the message to check.
   * @param {number} migrationBlock - The block number to compare against.
   * @returns {Promise<boolean>} `true` if the message was sent after the migration block, `false` otherwise.
   */
  public async isMessageSentAfterMigrationBlock(messageHash: string, migrationBlock: number): Promise<boolean> {
    const [messageSentEvent] = await this.l2EventLogClient.getMessageSentEventsByMessageHash({
      messageHash,
    });

    if (!messageSentEvent) {
      throw makeBaseError(`Message hash does not exist on L2. Message hash: ${messageHash}`);
    }

    if (migrationBlock > messageSentEvent.blockNumber) {
      return false;
    }

    return true;
  }

  /**
   * Attempts to find the block number at which a migration event occurred.
   *
   * @returns {Promise<number | null>} The block number of the migration event, or null if not found.
   */
  public async findMigrationBlock(): Promise<number | null> {
    const migrationBlock = this.cache.get(`migration-block-${this.network}`);

    if (migrationBlock) {
      return migrationBlock;
    }

    const [serviceVersionMigratedEvent] = await this.l2EventLogClient.getServiceVersionMigratedEvents();

    if (!serviceVersionMigratedEvent) {
      return null;
    }

    this.cache.set(`migration-block-${this.network}`, serviceVersionMigratedEvent.blockNumber);

    return serviceVersionMigratedEvent.blockNumber;
  }

  /**
   * Retrieves the proof for a given message hash.
   *
   * @param {string} messageHash - The hash of the message to get the proof for.
   * @returns {Promise<Proof>} The proof for the specified message hash.
   */
  public async getMessageProof(messageHash: string): Promise<Proof> {
    return this.l1ContractClient.getMessageProof(messageHash);
  }

  /**
   * Determines if claiming a message requires a proof.
   *
   * @param {string} messageHash - The hash of the message to check.
   * @returns {Promise<boolean>} `true` if claiming the message requires a proof, `false` otherwise.
   */
  public async isClaimingNeedingProof(messageHash: string): Promise<boolean> {
    if (this.network === "custom") {
      return true;
    }

    const migrationBlock = await this.findMigrationBlock();

    if (!migrationBlock) {
      return false;
    }

    return this.isMessageSentAfterMigrationBlock(messageHash, migrationBlock);
  }

  /**
   * Retrieves the on-chain status of a message.
   *
   * @param {string} messageHash - The hash of the message to check the status for.
   * @param {Overrides} [overrides={}] - Optional overrides for the contract call. Defaults to `{}` if not specified.
   * @returns {Promise<OnChainMessageStatus>} The on-chain status of the message.
   */
  public async getMessageStatus(messageHash: string, overrides: Overrides = {}): Promise<OnChainMessageStatus> {
    const isClaimingNeedingProof = await this.isClaimingNeedingProof(messageHash.toString());

    if (!isClaimingNeedingProof) {
      return this.getMessageStatusUsingMessageHash(messageHash, overrides);
    }

    return this.getMessageStatusUsingMerkleTree(messageHash, overrides);
  }

  /**
   * Retrieves the L2 message status on L1 using the message hash (for messages sent before migration).
   * @param {string} messageHash - The hash of the message sent on L2.
   * @param {Overrides} [overrides={}] - Ethers call overrides. Defaults to `{}` if not specified.
   * @returns {Promise<OnChainMessageStatus>} The on-chain status of the message.
   */
  private async getMessageStatusUsingMessageHash(
    messageHash: string,
    overrides: Overrides = {},
  ): Promise<OnChainMessageStatus> {
    return this.l1ContractClient.getMessageStatusUsingMessageHash(messageHash, overrides);
  }

  /**
   * Retrieves the L2 message status on L1 using merkle tree (for messages sent after migration).
   * @param {string} messageHash - The hash of the message sent on L2.
   * @param {Overrides} [overrides={}] - Ethers call overrides. Defaults to `{}` if not specified.
   * @returns {Promise<OnChainMessageStatus>} The on-chain status of the message.
   */
  private async getMessageStatusUsingMerkleTree(
    messageHash: string,
    overrides: Overrides = {},
  ): Promise<OnChainMessageStatus> {
    return this.l1ContractClient.getMessageStatusUsingMerkleTree(messageHash, overrides);
  }

  /**
   * Estimates the gas required to claim a message.
   *
   * @param {MessageSent & { feeRecipient?: string }} message - The message to estimate the claim gas for.
   * @param {Overrides} [overrides={}] - Optional overrides for the contract call. Defaults to `{}` if not specified.
   * @returns {Promise<bigint>} The estimated gas required to claim the message.
   */
  public async estimateClaimMessageGas(
    message: MessageSent & { feeRecipient?: string },
    overrides: Overrides = {},
  ): Promise<bigint> {
    const isClaimingNeedingProof = await this.isClaimingNeedingProof(message.messageHash);

    if (!isClaimingNeedingProof) {
      const gasLimit = await this.l1ContractClient.estimateClaimWithoutProofGas(message, overrides);
      return gasLimit;
    }

    const gasLimit = await this.l1ContractClient.estimateClaimGas(message, overrides);
    return gasLimit;
  }

  /**
   * Executes the claim transaction for a message.
   *
   * @param {MessageSent & { feeRecipient?: string }} message - The message to claim.
   * @param {Overrides} [overrides={}] - Optional overrides for the contract call. Defaults to `{}` if not specified.
   * @returns {Promise<ContractTransactionResponse>} The transaction response for the claim operation.
   */
  public async claimMessage(
    message: MessageSent & { feeRecipient?: string },
    overrides: Overrides = {},
  ): Promise<ContractTransactionResponse> {
    const isClaimingNeedingProof = await this.isClaimingNeedingProof(message.messageHash);

    if (!isClaimingNeedingProof) {
      return this.l1ContractClient.claimWithoutProof(message, overrides);
    }

    return this.l1ContractClient.claim(message, overrides) as Promise<ContractTransactionResponse>;
  }
}
