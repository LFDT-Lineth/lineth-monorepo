import { Message } from "../../types";
import { OnChainMessageStatus } from "../../enums";
import { IMessageServiceContract } from "../IMessageServiceContract";
import { FinalizationMessagingInfo, Proof } from "./IMerkleTreeService";

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
  getMessageStatusUsingMerkleTree(messageHash: string, overrides: Overrides): Promise<OnChainMessageStatus>;
  estimateClaimGas(message: Message & { feeRecipient?: string }, overrides?: Overrides): Promise<bigint>;
  estimateClaimWithoutProofGas(message: Message & { feeRecipient?: string }, overrides: Overrides): Promise<bigint>;
  claimWithoutProof(
    message: Message & { feeRecipient?: string },
    overrides: Overrides,
  ): Promise<ContractTransactionResponse>;
}
