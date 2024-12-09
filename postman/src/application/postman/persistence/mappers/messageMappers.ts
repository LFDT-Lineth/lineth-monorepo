import { Message } from "../../../../core/entities/Message";
import { MessageEntity } from "../entities/Message.entity";

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
    messageSender: entity.messageSender,
    destination: entity.destination,
    fee: BigInt(entity.fee),
    value: BigInt(entity.value),
    messageNonce: BigInt(entity.messageNonce),
    calldata: entity.calldata,
    messageHash: entity.messageHash,
    contractAddress: entity.messageContractAddress,
    sentBlockNumber: entity.sentBlockNumber,
    direction: entity.direction,
    status: entity.status,
    claimTxCreationDate: entity.claimTxCreationDate,
    claimTxGasLimit: entity.claimTxGasLimit,
    claimTxMaxFeePerGas: entity.claimTxMaxFeePerGas,
    claimTxMaxPriorityFeePerGas: entity.claimTxMaxPriorityFeePerGas,
    claimTxNonce: entity.claimTxNonce,
    claimTxHash: entity.claimTxHash,
    claimNumberOfRetry: entity.claimNumberOfRetry,
    claimLastRetriedAt: entity.claimLastRetriedAt,
    claimGasEstimationThreshold: entity.claimGasEstimationThreshold,
    compressedTransactionSize: entity.compressedTransactionSize,
    createdAt: entity.createdAt,
    updatedAt: entity.updatedAt,
  });
};
