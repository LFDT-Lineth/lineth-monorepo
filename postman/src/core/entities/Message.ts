import { Direction } from "../enums";
import { MessageStatus } from "../enums";

import type { Address, Hash, Hex } from "../types/primitives";

export type MessageProps = {
  id?: number;
  messageSender: Address;
  destination: Address;
  fee: bigint;
  value: bigint;
  messageNonce: bigint;
  calldata: Hex;
  messageHash: Hash;
  contractAddress: Address;
  sentBlockNumber: number;
  direction: Direction;
  status: MessageStatus;
  claimTxCreationDate?: Date;
  claimTxGasLimit?: number;
  claimTxMaxFeePerGas?: bigint;
  claimTxMaxPriorityFeePerGas?: bigint;
  claimTxNonce?: number;
  claimTxHash?: Hash;
  claimNumberOfRetry: number;
  claimCycleCount: number;
  claimLastRetriedAt?: Date;
  claimGasEstimationThreshold?: number;
  compressedTransactionSize?: number;
  isForSponsorship?: boolean;
  createdAt?: Date;
  updatedAt?: Date;
};

export type MessageWithProofProps = MessageProps & {
  proof: Hex[];
  leafIndex: number;
  merkleRoot: Hash;
};

type EditableMessageProps = Omit<
  MessageProps,
  | "id"
  | "messageSender"
  | "destination"
  | "fee"
  | "value"
  | "messageNonce"
  | "calldata"
  | "messageHash"
  | "contractAddress"
  | "sentBlockNumber"
  | "direction"
  | "createdAt"
  | "updatedAt"
>;

export class Message {
  public id?: number;
  public messageSender: Address;
  public destination: Address;
  public fee: bigint;
  public value: bigint;
  public messageNonce: bigint;
  public calldata: Hex;
  public messageHash: Hash;
  public contractAddress: Address;
  public sentBlockNumber: number;
  public direction: Direction;
  public status: MessageStatus;
  public claimTxCreationDate?: Date;
  public claimTxGasLimit?: number;
  public claimTxMaxFeePerGas?: bigint;
  public claimTxMaxPriorityFeePerGas?: bigint;
  public claimTxNonce?: number;
  public claimTxHash?: Hash;
  public claimNumberOfRetry: number;
  public claimCycleCount: number;
  public claimLastRetriedAt?: Date;
  public claimGasEstimationThreshold?: number;
  public compressedTransactionSize?: number;
  public isForSponsorship: boolean = false;
  public createdAt?: Date;
  public updatedAt?: Date;

  constructor(props: MessageProps) {
    this.id = props.id;
    this.messageSender = props.messageSender;
    this.destination = props.destination;
    this.fee = props.fee;
    this.value = props.value;
    this.messageNonce = props.messageNonce;
    this.calldata = props.calldata;
    this.messageHash = props.messageHash;
    this.contractAddress = props.contractAddress;
    this.sentBlockNumber = props.sentBlockNumber;
    this.direction = props.direction;
    this.status = props.status;
    this.claimTxCreationDate = props.claimTxCreationDate;
    this.claimTxGasLimit = props.claimTxGasLimit;
    this.claimTxMaxFeePerGas = props.claimTxMaxFeePerGas;
    this.claimTxMaxPriorityFeePerGas = props.claimTxMaxPriorityFeePerGas;
    this.claimTxNonce = props.claimTxNonce;
    this.claimTxHash = props.claimTxHash;
    this.claimNumberOfRetry = props.claimNumberOfRetry;
    this.claimCycleCount = props.claimCycleCount;
    this.claimLastRetriedAt = props.claimLastRetriedAt;
    this.claimGasEstimationThreshold = props.claimGasEstimationThreshold;
    this.compressedTransactionSize = props.compressedTransactionSize;
    this.isForSponsorship = props.isForSponsorship ?? false;
    this.createdAt = props.createdAt;
    this.updatedAt = props.updatedAt;
  }

  public hasZeroFee(): boolean {
    return this.fee === 0n;
  }

  public edit(newMessage: Partial<EditableMessageProps>) {
    if (newMessage.status !== undefined) this.status = newMessage.status;
    if (newMessage.claimTxCreationDate !== undefined) this.claimTxCreationDate = newMessage.claimTxCreationDate;
    if (newMessage.claimTxGasLimit !== undefined) this.claimTxGasLimit = newMessage.claimTxGasLimit;
    if (newMessage.claimTxMaxFeePerGas !== undefined) this.claimTxMaxFeePerGas = newMessage.claimTxMaxFeePerGas;
    if (newMessage.claimTxMaxPriorityFeePerGas !== undefined)
      this.claimTxMaxPriorityFeePerGas = newMessage.claimTxMaxPriorityFeePerGas;
    if (newMessage.claimTxNonce !== undefined) this.claimTxNonce = newMessage.claimTxNonce;
    if (newMessage.claimTxHash !== undefined) this.claimTxHash = newMessage.claimTxHash;
    if (newMessage.claimNumberOfRetry !== undefined) this.claimNumberOfRetry = newMessage.claimNumberOfRetry;
    if (newMessage.claimCycleCount !== undefined) this.claimCycleCount = newMessage.claimCycleCount;
    if (newMessage.claimLastRetriedAt !== undefined) this.claimLastRetriedAt = newMessage.claimLastRetriedAt;
    if (newMessage.claimGasEstimationThreshold !== undefined)
      this.claimGasEstimationThreshold = newMessage.claimGasEstimationThreshold;
    if (newMessage.compressedTransactionSize !== undefined)
      this.compressedTransactionSize = newMessage.compressedTransactionSize;
    if (newMessage.isForSponsorship !== undefined) this.isForSponsorship = newMessage.isForSponsorship;

    this.updatedAt = new Date();
  }

  public toString(): string {
    return `Message(messageSender=${this.messageSender}, destination=${this.destination}, fee=${
      this.fee
    }, value=${this.value}, messageNonce=${this.messageNonce}, calldata=${this.calldata}, messageHash=${
      this.messageHash
    }, contractAddress=${this.contractAddress}, sentBlockNumber=${this.sentBlockNumber}, direction=${
      this.direction
    }, status=${this.status}, claimTxCreationDate=${this.claimTxCreationDate?.toISOString()}, claimTxGasLimit=${
      this.claimTxGasLimit
    }, claimTxMaxFeePerGas=${this.claimTxMaxFeePerGas}, claimTxMaxPriorityFeePerGas=${
      this.claimTxMaxPriorityFeePerGas
    }, claimTxNonce=${this.claimTxNonce}, claimTransactionHash=${this.claimTxHash}, claimNumberOfRetry=${
      this.claimNumberOfRetry
    }, claimCycleCount=${this.claimCycleCount}, claimLastRetriedAt=${this.claimLastRetriedAt?.toISOString()}, claimGasEstimationThreshold=${
      this.claimGasEstimationThreshold
    }, compressedTransactionSize=${
      this.compressedTransactionSize
    }, isForSponsorship=${this.isForSponsorship}, createdAt=${this.createdAt?.toISOString()}, updatedAt=${this.updatedAt?.toISOString()})`;
  }
}
