import { LineaGasFees } from "../../clients/blockchain/IGasProvider";
import { MessageProps } from "../../entities/Message";

export interface IL2ClaimTransactionSizeCalculator {
  calculateTransactionSize(message: MessageProps & { feeRecipient?: string }, fees: LineaGasFees): Promise<number>;
}
