import { Message } from "../../entities/Message";
import { Direction } from "../../enums";
import { TransactionReceipt } from "../../types";

export interface IReceiptStatusResolver {
  resolveReceiptStatus(message: Message, receipt: TransactionReceipt, receiptReceivedAt: Date): Promise<void>;
}

export type ReceiptStatusResolverConfig = {
  direction: Direction;
};
