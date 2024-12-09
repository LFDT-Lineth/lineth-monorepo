import { Message } from "../../types";
import { IMessageServiceContract } from "../IMessageServiceContract";
import { LineaGasFees } from "../IGasProvider";

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
  encodeClaimMessageTransactionData(message: Message & { feeRecipient?: string }): string;
  estimateClaimGasFees(message: Message & { feeRecipient?: string }, overrides?: Overrides): Promise<LineaGasFees>;
  getSigner(): Signer | undefined;
  getContractAddress(): string;
}
