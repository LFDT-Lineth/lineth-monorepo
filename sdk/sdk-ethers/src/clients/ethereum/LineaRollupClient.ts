import {
  Overrides,
  ContractTransactionResponse,
  TransactionRequest,
  TransactionResponse,
  Signer,
  TransactionReceipt,
  Block,
  ErrorDescription,
} from "ethers";

import { LineaRollup, LineaRollup__factory } from "../../contracts/typechain";
import {
  ILineaRollupClient,
  ILineaRollupLogClient,
  FinalizationMessagingInfo,
  IMerkleTreeService,
  Proof,
} from "../../core/clients/ethereum";
import { GasFees, IEthereumGasProvider } from "../../core/clients/IGasProvider";
import { IMessageRetriever } from "../../core/clients/IMessageRetriever";
import { IProvider } from "../../core/clients/IProvider";
import { IL2MessageServiceLogClient } from "../../core/clients/linea";
import {
  MESSAGE_UNKNOWN_STATUS,
  MESSAGE_CLAIMED_STATUS,
  ZERO_ADDRESS,
  DEFAULT_RATE_LIMIT_MARGIN,
} from "../../core/constants";
import { OnChainMessageStatus } from "../../core/enums";
import { makeBaseError } from "../../core/errors/utils";
import { Message, SDKMode, MessageSent } from "../../core/types";
import { formatMessageStatus, isString } from "../../core/utils";
import { BrowserProvider, Provider } from "../providers";

export class LineaRollupClient implements ILineaRollupClient<
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  ContractTransactionResponse,
  ErrorDescription
> {
  private readonly contract: LineaRollup;

  /**
   * @notice Initializes a new instance of the `LineaRollupClient`.
   * @dev This constructor sets up the Linea Rollup Client with the necessary dependencies and configurations.
   * @param {ethers.Provider} provider The provider for interacting with the blockchain.
   * @param {string} contractAddress The address of the Linea Rollup contract.
   * @param {ILineaRollupLogClient} lineaRollupLogClient An instance of a class implementing the `ILineaRollupLogClient` interface for fetching events from the blockchain.
   * @param {IL2MessageServiceLogClient} l2MessageServiceLogClient An instance of a class implementing the `IL2MessageServiceLogClient` interface for fetching events from the blockchain.
   * @param {IEthereumGasProvider} gasProvider An instance of a class implementing the `IEthereumGasProvider` interface for providing gas estimates.
   * @param {IMessageRetriever} messageRetriever An instance of a class implementing the `IMessageRetriever` interface for retrieving messages.
   * @param {IMerkleTreeService} merkleTreeService An instance of a class implementing the `IMerkleTreeService` interface for managing Merkle trees.
   * @param {SDKMode} mode The mode in which the SDK is operating, e.g., `read-only` or `read-write`.
   * @param {Signer} signer An optional Ethers.js signer object for signing transactions.
   */
  constructor(
    private readonly provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      Provider | BrowserProvider
    >,
    private readonly contractAddress: string,
    private readonly lineaRollupLogClient: ILineaRollupLogClient,
    private readonly l2MessageServiceLogClient: IL2MessageServiceLogClient,
    private readonly gasProvider: IEthereumGasProvider<TransactionRequest>,
    private readonly messageRetriever: IMessageRetriever<TransactionReceipt>,
    private readonly merkleTreeService: IMerkleTreeService,
    private readonly mode: SDKMode,
    private readonly signer?: Signer,
  ) {
    this.contract = this.getContract(this.contractAddress, this.signer);
  }

  /**
   * Retrieves message information by message hash.
   * @param {string} messageHash - The hash of the message sent on L1.
   * @returns {Promise<MessageSent | null>} The message information or null if not found.
   */
  public async getMessageByMessageHash(messageHash: string): Promise<MessageSent | null> {
    return this.messageRetriever.getMessageByMessageHash(messageHash);
  }

  /**
   * Retrieves messages information by the transaction hash.
   * @param {string} transactionHash - The hash of the `sendMessage` transaction on L1.
   * @returns {Promise<MessageSent[] | null>} An array of message information or null if not found.
   */
  public async getMessagesByTransactionHash(transactionHash: string): Promise<MessageSent[] | null> {
    return this.messageRetriever.getMessagesByTransactionHash(transactionHash);
  }

  /**
   * Retrieves the transaction receipt by message hash.
   * @param {string} messageHash - The hash of the message sent on L1.
   * @returns {Promise<TransactionReceipt | null>} The transaction receipt or null if not found.
   */
  public async getTransactionReceiptByMessageHash(messageHash: string): Promise<TransactionReceipt | null> {
    return this.messageRetriever.getTransactionReceiptByMessageHash(messageHash);
  }

  /**
   * Retrieves the finalization messaging info. This function is used in the L1 claiming flow.
   * @param {string} transactionHash - The finalization transaction hash.
   * @returns {Promise<FinalizationMessagingInfo>} The finalization messaging info: l2MessagingBlocksRange, l2MerkleRoots, treeDepth.
   */
  public async getFinalizationMessagingInfo(transactionHash: string): Promise<FinalizationMessagingInfo> {
    return this.merkleTreeService.getFinalizationMessagingInfo(transactionHash);
  }

  /**
   * Retrieves L2 message hashes in a specific L2 block range.
   * @param {number} fromBlock - The starting block number.
   * @param {number} toBlock - The ending block number.
   * @returns {Promise<string[]>} A list of all L2 message hashes in the specified block range.
   */
  public async getL2MessageHashesInBlockRange(fromBlock: number, toBlock: number): Promise<string[]> {
    return this.merkleTreeService.getL2MessageHashesInBlockRange(fromBlock, toBlock);
  }

  /**
   * Retrieves message siblings for building the merkle tree. This merkle tree will be used to generate the proof to be able to claim the message on L1.
   * @param {string} messageHash - The message hash.
   * @param {string[]} messageHashes - The list of all L2 message hashes finalized in the same finalization transaction on L1.
   * @param {number} treeDepth - The merkle tree depth.
   * @returns {string[]} The message siblings.
   */
  public getMessageSiblings(messageHash: string, messageHashes: string[], treeDepth: number): string[] {
    return this.merkleTreeService.getMessageSiblings(messageHash, messageHashes, treeDepth);
  }

  /**
   * Retrieves the message proof for claiming the message on L1.
   * @param {string} messageHash - The message hash.
   * @param {number} messageBlockNumber - The L2 block number where the message was sent. Defaults to `undefined`.
   * @returns {Promise<Proof>} The merkle root, the merkle proof and the message leaf index.
   */
  public async getMessageProof(messageHash: string, messageBlockNumber?: number): Promise<Proof> {
    return this.merkleTreeService.getMessageProof(messageHash, messageBlockNumber);
  }

  public async getGasFees(): Promise<GasFees> {
    return this.gasProvider.getGasFees();
  }

  public getMaxFeePerGas(): bigint {
    return this.gasProvider.getMaxFeePerGas();
  }

  /**
   * Retrieves the LineaRollup contract instance.
   * @param {string} contractAddress - Address of the L1 contract.
   * @param {Signer} [signer] - The signer instance.
   * @returns {LineaRollup} The LineaRollup contract instance.
   */
  private getContract(contractAddress: string, signer?: Signer): LineaRollup {
    if (this.mode === "read-only") {
      return LineaRollup__factory.connect(contractAddress, this.provider);
    }

    if (!signer) {
      throw makeBaseError("Please provide a signer.");
    }

    return LineaRollup__factory.connect(contractAddress, signer);
  }

  /**
   * Retrieves the L2 message status on L1 using the message hash (for messages sent before migration).
   * @param {string} messageHash - The hash of the message sent on L2.
   * @param {Overrides} [overrides={}] - Ethers call overrides. Defaults to `{}` if not specified.
   * @returns {Promise<OnChainMessageStatus>} The message status (CLAIMED, CLAIMABLE, UNKNOWN).
   */
  public async getMessageStatusUsingMessageHash(
    messageHash: string,
    overrides: Overrides = {},
  ): Promise<OnChainMessageStatus> {
    let status = await this.contract.inboxL2L1MessageStatus(messageHash, overrides);
    if (status === BigInt(MESSAGE_UNKNOWN_STATUS)) {
      const events = await this.lineaRollupLogClient.getMessageClaimedEvents({
        filters: { messageHash },
        fromBlock: 0,
        toBlock: "latest",
      });
      if (events.length > 0) {
        status = BigInt(MESSAGE_CLAIMED_STATUS);
      }
    }
    return formatMessageStatus(status);
  }

  /**
   * Retrieves the L2 message status on L1.
   * @param {string} params - The parameters object.
   * @param {string} params.messageHash - The hash of the message sent on L2.
   * @param {number} [params.messageBlockNumber] - The L2 block number where the message was sent. Defaults to `undefined`.
   * @param {Overrides} [params.overrides={}] - Ethers call overrides. Defaults to `{}` if not specified.
   * @returns {Promise<OnChainMessageStatus>} The message status (CLAIMED, CLAIMABLE, UNKNOWN).
   */
  public async getMessageStatus(params: {
    messageHash: string;
    messageBlockNumber?: number;
    overrides?: Overrides;
  }): Promise<OnChainMessageStatus> {
    const { messageHash, messageBlockNumber, overrides = {} } = params;
    return this.getMessageStatusUsingMerkleTree({ messageHash, messageBlockNumber, overrides });
  }

  /**
   * Retrieves the L2 message status on L1 using merkle tree (for messages sent after migration).
   * @param {string} messageHash - The hash of the message sent on L2.
   * @param {number} [messageBlockNumber] - The L2 block number where the message was sent. Defaults to `undefined`.
   * @param {Overrides} [overrides={}] - Ethers call overrides. Defaults to `{}` if not specified.
   * @returns {Promise<OnChainMessageStatus>} The message status (CLAIMED, CLAIMABLE, UNKNOWN).
   */
  public async getMessageStatusUsingMerkleTree(params: {
    messageHash: string;
    messageBlockNumber?: number;
    overrides?: Overrides;
  }): Promise<OnChainMessageStatus> {
    const { messageHash, messageBlockNumber, overrides = {} } = params;
    const [messageEvent] = await this.l2MessageServiceLogClient.getMessageSentEventsByMessageHash({
      messageHash,
      fromBlock: messageBlockNumber,
      toBlock: messageBlockNumber,
    });

    if (!messageEvent) {
      throw makeBaseError(`Message hash does not exist on L2. Message hash: ${messageHash}`);
    }

    const [[l2MessagingBlockAnchoredEvent], isMessageClaimed] = await Promise.all([
      this.lineaRollupLogClient.getL2MessagingBlockAnchoredEvents({
        filters: { l2Block: BigInt(messageEvent.blockNumber) },
      }),
      this.contract.isMessageClaimed(messageEvent.messageNonce, overrides),
    ]);

    if (isMessageClaimed) {
      return OnChainMessageStatus.CLAIMED;
    }

    if (l2MessagingBlockAnchoredEvent) {
      return OnChainMessageStatus.CLAIMABLE;
    }

    return OnChainMessageStatus.UNKNOWN;
  }

  /**
   * Estimates the gas required for the claimMessage transaction.
   * @param {Message & { feeRecipient?: string }} message - The message information.
   * @param {Overrides} [opts={}] - Claiming options and Ethers payable overrides. Defaults to `{}` if not specified.
   * @returns {Promise<bigint>} The estimated transaction gas.
   */
  public async estimateClaimWithoutProofGas(
    message: Message & { feeRecipient?: string },
    opts: {
      claimViaAddress?: string;
      overrides?: Overrides;
    } = {},
  ): Promise<bigint> {
    if (this.mode === "read-only") {
      throw makeBaseError("'EstimateClaimGas' function not callable using readOnly mode.");
    }

    const { messageSender, destination, fee, value, calldata, messageNonce, feeRecipient } = message;
    const l1FeeRecipient = feeRecipient ?? ZERO_ADDRESS;

    const claimingContract = opts.claimViaAddress ? this.getContract(opts.claimViaAddress, this.signer) : this.contract;

    try {
      return await claimingContract.claimMessage.estimateGas(
        messageSender,
        destination,
        fee,
        value,
        l1FeeRecipient,
        calldata,
        messageNonce,
        {
          ...(await this.gasProvider.getGasFees()),
          ...opts.overrides,
        },
      );
    } catch (e) {
      throw makeBaseError(e, message);
    }
  }

  /**
   * Claims the message on L1 without merkle tree (for message sent before the migration).
   * @param {Message & { feeRecipient?: string }} message - The message information.
   * @param {Overrides} [opts={}] - Claiming options and Ethers payable overrides. Defaults to `{}` if not specified.
   * @returns {Promise<ContractTransactionResponse>} The transaction response.
   */
  public async claimWithoutProof(
    message: Message & { feeRecipient?: string },
    opts: {
      claimViaAddress?: string;
      overrides?: Overrides;
    } = {},
  ): Promise<ContractTransactionResponse> {
    if (this.mode === "read-only") {
      throw makeBaseError("'claim' function not callable using readOnly mode.");
    }

    const { messageSender, destination, fee, value, calldata, messageNonce, feeRecipient } = message;
    const l1FeeRecipient = feeRecipient ?? ZERO_ADDRESS;

    const claimingContract = opts.claimViaAddress ? this.getContract(opts.claimViaAddress, this.signer) : this.contract;

    return await claimingContract.claimMessage(
      messageSender,
      destination,
      fee,
      value,
      l1FeeRecipient,
      calldata,
      messageNonce,
      {
        ...(await this.gasProvider.getGasFees()),
        ...opts.overrides,
      },
    );
  }

  /**
   * Estimates the gas required for the claimMessageWithProof transaction.
   * @param {Message & { feeRecipient?: string; messageBlockNumber?: number }} message - The message information.
   * @param {Overrides} [opts={}] - Claiming options and Ethers payable overrides. Defaults to `{}` if not specified.
   * @returns {Promise<bigint>} The estimated gas.
   */
  public async estimateClaimGas(
    message: Message & { feeRecipient?: string; messageBlockNumber?: number },
    opts: {
      claimViaAddress?: string;
      overrides?: Overrides;
    } = {},
  ): Promise<bigint> {
    if (this.mode === "read-only") {
      throw makeBaseError("'EstimateClaimGasFees' function not callable using readOnly mode.");
    }

    const { messageSender, destination, fee, value, calldata, messageNonce, feeRecipient } = message;

    const { proof, leafIndex, root } = await this.merkleTreeService.getMessageProof(
      message.messageHash,
      message.messageBlockNumber,
    );

    const l1FeeRecipient = feeRecipient ?? ZERO_ADDRESS;

    const claimingContract = opts.claimViaAddress ? this.getContract(opts.claimViaAddress, this.signer) : this.contract;

    try {
      return await claimingContract.claimMessageWithProof.estimateGas(
        {
          from: messageSender,
          to: destination,
          fee,
          value,
          data: calldata,
          messageNumber: messageNonce,
          proof,
          leafIndex,
          merkleRoot: root,
          feeRecipient: l1FeeRecipient,
        },

        {
          ...(await this.gasProvider.getGasFees()),
          ...opts.overrides,
        },
      );
    } catch (e) {
      throw makeBaseError(e, message);
    }
  }

  /**
   * Claims the message using merkle proof on L1.
   * @param {Message & { feeRecipient?: string; messageBlockNumber?: number }} message - The message information.
   * @param {Overrides} [opts={}] - Claiming options and Ethers payable overrides. Defaults to `{}` if not specified.
   * @returns {Promise<ContractTransactionResponse>} The transaction response.
   */
  public async claim(
    message: Message & { feeRecipient?: string; messageBlockNumber?: number },
    opts: {
      claimViaAddress?: string;
      overrides?: Overrides;
    } = {},
  ): Promise<ContractTransactionResponse> {
    if (this.mode === "read-only") {
      throw makeBaseError("'claim' function not callable using readOnly mode.");
    }

    const { messageSender, destination, fee, value, calldata, messageNonce, feeRecipient } = message;

    const l1FeeRecipient = feeRecipient ?? ZERO_ADDRESS;

    const { proof, leafIndex, root } = await this.merkleTreeService.getMessageProof(
      message.messageHash,
      message.messageBlockNumber,
    );

    const claimingContract = opts.claimViaAddress ? this.getContract(opts.claimViaAddress, this.signer) : this.contract;

    return await claimingContract.claimMessageWithProof(
      {
        from: messageSender,
        to: destination,
        fee,
        value,
        data: calldata,
        messageNumber: messageNonce,
        proof,
        leafIndex,
        merkleRoot: root,
        feeRecipient: l1FeeRecipient,
      },
      {
        ...(await this.gasProvider.getGasFees()),
        ...opts.overrides,
      },
    );
  }

  /**
   * Retries a specific transaction with a higher fee.
   * @param {string} transactionHash - The transaction hash.
   * @param {number} [priceBumpPercent=10] - The percentage of price increase. Defaults to `10` if not specified.
   * @returns {Promise<TransactionResponse>} The transaction response.
   */
  public async retryTransactionWithHigherFee(
    transactionHash: string,
    priceBumpPercent: number = 10,
  ): Promise<TransactionResponse> {
    if (!Number.isInteger(priceBumpPercent)) {
      throw makeBaseError("'priceBumpPercent' must be an integer");
    }

    if (this.mode === "read-only") {
      throw makeBaseError("'retryTransactionWithHigherFee' function not callable using readOnly mode.");
    }

    const transaction = await this.provider.getTransaction(transactionHash);

    if (!transaction) {
      throw makeBaseError(`Transaction with hash ${transactionHash} not found.`);
    }

    let maxPriorityFeePerGas;
    let maxFeePerGas;

    if (!transaction.maxPriorityFeePerGas || !transaction.maxFeePerGas) {
      const txFees = await this.gasProvider.getGasFees();
      maxPriorityFeePerGas = txFees.maxPriorityFeePerGas;
      maxFeePerGas = txFees.maxFeePerGas;
    } else {
      maxPriorityFeePerGas = (transaction.maxPriorityFeePerGas * (BigInt(priceBumpPercent) + 100n)) / 100n;
      maxFeePerGas = (transaction.maxFeePerGas * (BigInt(priceBumpPercent) + 100n)) / 100n;
      const maxFeePerGasFromConfig = this.gasProvider.getMaxFeePerGas();
      if (maxPriorityFeePerGas > maxFeePerGasFromConfig) {
        maxPriorityFeePerGas = maxFeePerGasFromConfig;
      }
      if (maxFeePerGas > maxFeePerGasFromConfig) {
        maxFeePerGas = maxFeePerGasFromConfig;
      }
    }

    const updatedTransaction: TransactionRequest = {
      to: transaction.to,
      value: transaction.value,
      data: transaction.data,
      nonce: transaction.nonce,
      gasLimit: transaction.gasLimit,
      chainId: transaction.chainId,
      type: 2,
      maxPriorityFeePerGas,
      maxFeePerGas,
    };
    const signedTransaction = await this.signer!.signTransaction(updatedTransaction);
    return await this.provider.broadcastTransaction(signedTransaction);
  }

  /**
   * Checks if the withdrawal rate limit has been exceeded.
   * @param {bigint} messageFee - The message fee.
   * @param {bigint} messageValue - The message value.
   * @returns {Promise<boolean>} True if the rate limit has been exceeded, false otherwise.
   */
  public async isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean> {
    const rateLimitInWei = await this.contract.limitInWei();
    const currentPeriodAmountInWei = await this.contract.currentPeriodAmountInWei();

    return (
      parseFloat((currentPeriodAmountInWei + BigInt(messageFee) + BigInt(messageValue)).toString()) >
      parseFloat(rateLimitInWei.toString()) * DEFAULT_RATE_LIMIT_MARGIN
    );
  }

  /**
   * Parses the error from the transaction.
   * @param {string} transactionHash - The transaction hash.
   * @returns {Promise<ErrorDescription | string>} The error description or the error bytes.
   */
  public async parseTransactionError(transactionHash: string): Promise<ErrorDescription | string> {
    let errorEncodedData = "0x";
    try {
      const tx = await this.provider.getTransaction(transactionHash);
      errorEncodedData = await this.provider.call({
        to: tx?.to,
        from: tx?.from,
        nonce: tx?.nonce,
        gasLimit: tx?.gasLimit,
        data: tx?.data,
        value: tx?.value,
        chainId: tx?.chainId,
        accessList: tx?.accessList,
        maxPriorityFeePerGas: tx?.maxPriorityFeePerGas,
        maxFeePerGas: tx?.maxFeePerGas,
      });
      const error = this.contract.interface.parseError(errorEncodedData);

      if (!error) {
        return errorEncodedData;
      }

      return error;
    } catch {
      return errorEncodedData;
    }
  }

  /**
   * Checks if an error is of type 'RateLimitExceeded'.
   * @param {string} transactionHash - The transaction hash.
   * @returns {Promise<boolean>} True if the error type is 'RateLimitExceeded', false otherwise.
   */
  public async isRateLimitExceededError(transactionHash: string): Promise<boolean> {
    const parsedError = await this.parseTransactionError(transactionHash);

    if (isString(parsedError)) {
      return false;
    }

    return parsedError.name === "RateLimitExceeded";
  }
}
