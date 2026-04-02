import { Message } from "../../../core/entities/Message";
import { MessageEntity } from "../entities/Message.entity";

import type { Address, Hash, Hex } from "../../../core/types/primitives";

export const mapMessageToMessageEntity = (message: Message): MessageEntity => {
  return {
    id: message?.id as number,
    ...message,
    fee: message.fee.toString(),
    value: message.value.toString(),
    messageNonce: parseInt(message.messageNonce.toString()),
    messageContractAddress: message.contractAddress,
    createdAt: message.createdAt ?? new Date(),
    updatedAt: message.updatedAt ?? new Date(),
  };
};

export const mapMessageEntityToMessage = (entity: MessageEntity): Message => {
  return new Message({
    id: entity.id,
    messageSender: entity.messageSender as Address,
    destination: entity.destination as Address,
    fee: BigInt(entity.fee),
    value: BigInt(entity.value),
    messageNonce: BigInt(entity.messageNonce),
    calldata: entity.calldata as Hex,
    messageHash: entity.messageHash as Hash,
    contractAddress: entity.messageContractAddress as Address,
    sentBlockNumber: entity.sentBlockNumber,
    direction: entity.direction,
    status: entity.status,
    claimTxCreationDate: entity.claimTxCreationDate,
    claimTxGasLimit: entity.claimTxGasLimit,
    claimTxMaxFeePerGas: entity.claimTxMaxFeePerGas,
    claimTxMaxPriorityFeePerGas: entity.claimTxMaxPriorityFeePerGas,
    claimTxNonce: entity.claimTxNonce,
    claimTxHash: entity.claimTxHash as Hash | undefined,
    claimNumberOfRetry: entity.claimNumberOfRetry,
    claimCycleCount: entity.claimCycleCount ?? 0,
    claimLastRetriedAt: entity.claimLastRetriedAt,
    claimGasEstimationThreshold: entity.claimGasEstimationThreshold,
    compressedTransactionSize: entity.compressedTransactionSize,
    isForSponsorship: entity.isForSponsorship,
    createdAt: entity.createdAt,
    updatedAt: entity.updatedAt,
  });
};
