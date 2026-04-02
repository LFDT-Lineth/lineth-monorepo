import { claimOnL1, getL2ToL1MessageStatus, getMessageProof as sdkGetMessageProof } from "@consensys/linea-sdk-viem";
import { type PublicClient, type WalletClient, decodeErrorResult } from "viem";
import { estimateContractGas, readContract } from "viem/actions";

import { ILineaRollupClient } from "../../../../core/clients/blockchain/ethereum/ILineaRollupClient";
import { Proof } from "../../../../core/clients/blockchain/ethereum/IMerkleTreeService";
import { IEthereumGasProvider } from "../../../../core/clients/blockchain/IGasProvider";
import { ZERO_ADDRESS } from "../../../../core/constants/blockchain";
import { DEFAULT_RATE_LIMIT_MARGIN } from "../../../../core/constants/common";
import { MessageProps } from "../../../../core/entities/Message";
import { OnChainMessageStatus } from "../../../../core/enums";
import { Address, Hash, ErrorDescription, MessageSent, Overrides, TransactionSubmission } from "../../../../core/types";
import { LineaRollupAbi } from "../../abis/LineaRollupAbi";

export class ViemLineaRollupClient implements ILineaRollupClient {
  constructor(
    private readonly publicClient: PublicClient,
    private readonly walletClient: WalletClient,
    private readonly contractAddress: Address,
    private readonly l2PublicClient: PublicClient,
    private readonly l2ContractAddress: Address,
    private readonly gasProvider: IEthereumGasProvider,
  ) {}

  public async getMessageStatus(params: {
    messageHash: Hash;
    messageBlockNumber?: number;
  }): Promise<OnChainMessageStatus> {
    return getL2ToL1MessageStatus(this.publicClient, {
      l2Client: this.l2PublicClient,
      messageHash: params.messageHash,
      lineaRollupAddress: this.contractAddress,
      l2MessageServiceAddress: this.l2ContractAddress,
      l2LogsBlockRange: params.messageBlockNumber
        ? { fromBlock: BigInt(params.messageBlockNumber), toBlock: BigInt(params.messageBlockNumber) }
        : undefined,
    });
  }

  public async getMessageProof(messageHash: Hash, messageBlockNumber?: number): Promise<Proof> {
    return sdkGetMessageProof(this.publicClient, {
      l2Client: this.l2PublicClient,
      messageHash,
      lineaRollupAddress: this.contractAddress,
      l2MessageServiceAddress: this.l2ContractAddress,
      l2LogsBlockRange: messageBlockNumber
        ? { fromBlock: BigInt(messageBlockNumber), toBlock: BigInt(messageBlockNumber) }
        : undefined,
    });
  }

  public async estimateClaimGas(
    message: (MessageSent | MessageProps) & { feeRecipient?: Address; messageBlockNumber?: number },
    opts: { claimViaAddress?: Address; overrides?: Overrides } = {},
  ): Promise<bigint> {
    const { proof, leafIndex, root } = await this.getMessageProof(
      message.messageHash,
      "messageBlockNumber" in message ? message.messageBlockNumber : undefined,
    );

    const feeRecipient = message.feeRecipient ?? ZERO_ADDRESS;

    const gasFees = await this.gasProvider.getGasFees();
    const contractAddress = opts.claimViaAddress ?? this.contractAddress;
    const [account] = await this.walletClient.getAddresses();

    return estimateContractGas(this.publicClient, {
      address: contractAddress,
      abi: LineaRollupAbi,
      functionName: "claimMessageWithProof",
      args: [
        {
          proof,
          messageNumber: message.messageNonce,
          leafIndex,
          from: message.messageSender,
          to: message.destination,
          fee: message.fee,
          value: message.value,
          feeRecipient,
          merkleRoot: root,
          data: message.calldata,
        },
      ],
      account,
      maxFeePerGas: opts.overrides?.maxFeePerGas ?? gasFees.maxFeePerGas,
      maxPriorityFeePerGas: opts.overrides?.maxPriorityFeePerGas ?? gasFees.maxPriorityFeePerGas,
    });
  }

  public async claim(
    message: (MessageSent | MessageProps) & { feeRecipient?: Address; messageBlockNumber?: number },
    opts: { claimViaAddress?: Address; overrides?: Overrides } = {},
  ): Promise<TransactionSubmission> {
    const { proof, leafIndex, root } = await this.getMessageProof(
      message.messageHash,
      "messageBlockNumber" in message ? message.messageBlockNumber : undefined,
    );

    const feeRecipient = message.feeRecipient ?? ZERO_ADDRESS;

    const gasFees = await this.gasProvider.getGasFees();
    const contractAddress = opts.claimViaAddress ?? this.contractAddress;

    const txHash = await claimOnL1(this.walletClient, {
      from: message.messageSender,
      to: message.destination,
      fee: message.fee,
      value: message.value,
      messageNonce: message.messageNonce,
      calldata: message.calldata,
      feeRecipient,
      messageProof: { proof, root, leafIndex },
      lineaRollupAddress: contractAddress,
      nonce: opts.overrides?.nonce,
      gas: opts.overrides?.gasLimit,
      maxFeePerGas: opts.overrides?.maxFeePerGas ?? gasFees.maxFeePerGas,
      maxPriorityFeePerGas: opts.overrides?.maxPriorityFeePerGas ?? gasFees.maxPriorityFeePerGas,
    });

    return {
      hash: txHash,
      nonce: opts.overrides?.nonce ?? 0,
      gasLimit: opts.overrides?.gasLimit ?? 0n,
      maxFeePerGas: opts.overrides?.maxFeePerGas ?? gasFees.maxFeePerGas,
      maxPriorityFeePerGas: opts.overrides?.maxPriorityFeePerGas ?? gasFees.maxPriorityFeePerGas,
    };
  }

  public async isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean> {
    const [limitInWei, currentPeriodAmountInWei] = await Promise.all([
      readContract(this.publicClient, {
        address: this.contractAddress,
        abi: LineaRollupAbi,
        functionName: "limitInWei",
      }),
      readContract(this.publicClient, {
        address: this.contractAddress,
        abi: LineaRollupAbi,
        functionName: "currentPeriodAmountInWei",
      }),
    ]);

    return (
      parseFloat((currentPeriodAmountInWei + messageFee + messageValue).toString()) >
      parseFloat(limitInWei.toString()) * DEFAULT_RATE_LIMIT_MARGIN
    );
  }

  public async isRateLimitExceededError(transactionHash: Hash, blockNumber?: bigint): Promise<boolean> {
    const parsedError = await this.parseTransactionError(transactionHash, blockNumber);
    if (typeof parsedError === "string") return false;
    return parsedError.name === "RateLimitExceeded";
  }

  public async parseTransactionError(transactionHash: Hash, blockNumber?: bigint): Promise<ErrorDescription | string> {
    let errorEncodedData: Hash = "0x";
    try {
      const tx = await this.publicClient.getTransaction({ hash: transactionHash });
      if (!tx) return errorEncodedData;

      try {
        const { data } = await this.publicClient.call({
          to: tx.to ?? undefined,
          account: tx.from,
          nonce: tx.nonce,
          gas: tx.gas,
          data: tx.input,
          value: tx.value,
          maxFeePerGas: tx.maxFeePerGas ?? undefined,
          maxPriorityFeePerGas: tx.maxPriorityFeePerGas ?? undefined,
          // Replay at the original block so state-dependent checks (e.g. rate limits)
          // reflect the conditions that caused the revert, not the current block.
          blockNumber,
        });
        errorEncodedData = data ?? "0x";
      } catch (callError) {
        // Extract revert data from the call error if possible
        const errData = (callError as { data?: `0x${string}` }).data;
        if (errData) errorEncodedData = errData;
      }

      if (errorEncodedData === "0x") return errorEncodedData;

      const decoded = decodeErrorResult({
        abi: LineaRollupAbi,
        data: errorEncodedData,
      });

      return {
        name: decoded.errorName,
        args: decoded.args ? [...decoded.args] : [],
      };
    } catch {
      return errorEncodedData;
    }
  }
}
