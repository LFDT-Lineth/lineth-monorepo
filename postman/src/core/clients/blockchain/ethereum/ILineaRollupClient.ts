import { MessageSent, OnChainMessageStatus } from "@consensys/linea-sdk";

import { FinalizationMessagingInfo, Proof } from "./IMerkleTreeService";
import { MessageProps } from "../../../entities/Message";
import { IMessageServiceContract } from "../../../services/contracts/IMessageServiceContract";

export interface ILineaRollupClient<
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  ContractTransactionResponse,
  ErrorDescription,
> extends IMessageServiceContract<
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  ContractTransactionResponse,
  ErrorDescription
> {
  getFinalizationMessagingInfo(transactionHash: string): Promise<FinalizationMessagingInfo>;
  getL2MessageHashesInBlockRange(fromBlock: number, toBlock: number): Promise<string[]>;
  getMessageSiblings(messageHash: string, messageHashes: string[], treeDepth: number): string[];
  getMessageProof(messageHash: string): Promise<Proof>;
  getMessageStatusUsingMessageHash(messageHash: string, overrides: Overrides): Promise<OnChainMessageStatus>;
  getMessageStatusUsingMerkleTree(params: {
    messageHash: string;
    messageBlockNumber?: number;
    overrides?: Overrides;
  }): Promise<OnChainMessageStatus>;
  estimateClaimGas(
    message: (MessageSent | MessageProps) & { feeRecipient?: string; messageBlockNumber?: number },
    opts?: {
      claimViaAddress?: string;
      overrides?: Overrides;
    },
  ): Promise<bigint>;
  estimateClaimWithoutProofGas(
    message: (MessageSent | MessageProps) & { feeRecipient?: string },
    opts?: {
      claimViaAddress?: string;
      overrides?: Overrides;
    },
  ): Promise<bigint>;
  claimWithoutProof(
    message: (MessageSent | MessageProps) & { feeRecipient?: string },
    opts?: {
      claimViaAddress?: string;
      overrides?: Overrides;
    },
  ): Promise<ContractTransactionResponse>;
}
