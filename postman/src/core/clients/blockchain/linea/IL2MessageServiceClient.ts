import { MessageSent } from "@consensys/linea-sdk";
import { MessageProps } from "../../../entities/Message";
import { IMessageServiceContract } from "../../../services/contracts/IMessageServiceContract";
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
  encodeClaimMessageTransactionData(message: MessageProps & { feeRecipient?: string }): string;
  estimateClaimGasFees(
    message: (MessageSent | MessageProps) & { feeRecipient?: string },
    overrides?: Overrides,
  ): Promise<LineaGasFees>;
  getSigner(): Signer | undefined;
  getContractAddress(): string;
}
