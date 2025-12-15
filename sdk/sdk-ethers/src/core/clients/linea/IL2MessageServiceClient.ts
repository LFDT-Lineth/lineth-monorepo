import { Message } from "../../types";
import { IMessageServiceContract } from "../IMessageServiceContract";
import { LineaGasFees } from "../IGasProvider";
import { OnChainMessageStatus } from "../../enums";

export interface IL2MessageServiceClient<
  Overrides,
  TransactionReceipt,
  TransactionResponse,
  ContractTransactionResponse,
  Signer,
  ErrorDescription,
> extends IMessageServiceContract<
    Overrides,
    TransactionReceipt,
    TransactionResponse,
    ContractTransactionResponse,
    ErrorDescription
  > {
  getMessageStatus(params: { messageHash: string; overrides?: Overrides }): Promise<OnChainMessageStatus>;
  encodeClaimMessageTransactionData(message: Message & { feeRecipient?: string }): string;
  estimateClaimGasFees(
    message: Message & { feeRecipient?: string },
    opts?: {
      claimViaAddress?: string;
      overrides?: Overrides;
    },
  ): Promise<LineaGasFees>;
  getSigner(): Signer | undefined;
  getContractAddress(): string;
}
