/* eslint-disable @typescript-eslint/no-explicit-any */
import { Direction } from "@consensys/linea-sdk";

import { TEST_ADDRESS_1, TEST_CONTRACT_ADDRESS_1, TEST_CONTRACT_ADDRESS_2, TEST_MESSAGE_HASH } from "./constants";
import { MessageEntity } from "../../application/postman/persistence/entities/Message.entity";
import { Message, MessageProps } from "../../core/entities/Message";
import { MessageStatus } from "../../core/enums";
import { IPostmanLogger } from "../IPostmanLogger";

export class TestLogger implements IPostmanLogger {
  public readonly name: string;

  constructor(loggerName: string) {
    this.name = loggerName;
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  public info(error: any): void {}

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  public error(error: any): void {}

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  public warn(error: any): void {}

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  public debug(error: any): void {}

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  public warnOrError(error: any): void {}
}

export const generateMessage = (overrides?: Partial<MessageProps>): Message => {
  return new Message({
    id: 1,
    messageSender: TEST_ADDRESS_1,
    destination: TEST_CONTRACT_ADDRESS_1,
    fee: 10n,
    value: 2n,
    messageNonce: 1n,
    calldata: "0x",
    messageHash: TEST_MESSAGE_HASH,
    contractAddress: TEST_CONTRACT_ADDRESS_2,
    sentBlockNumber: 100_000,
    direction: Direction.L1_TO_L2,
    status: MessageStatus.SENT,
    claimNumberOfRetry: 0,
    isForSponsorship: false,
    createdAt: new Date("2023-08-04"),
    updatedAt: new Date("2023-08-04"),
    ...overrides,
  });
};

export const generateMessageEntity = (overrides?: Partial<MessageEntity>): MessageEntity => {
  return {
    id: 1,
    messageSender: TEST_ADDRESS_1,
    destination: TEST_CONTRACT_ADDRESS_1,
    fee: "10",
    value: "2",
    messageNonce: 1,
    calldata: "0x",
    messageHash: TEST_MESSAGE_HASH,
    messageContractAddress: TEST_CONTRACT_ADDRESS_2,
    sentBlockNumber: 100_000,
    direction: Direction.L1_TO_L2,
    status: MessageStatus.SENT,
    claimNumberOfRetry: 0,
    isForSponsorship: false,
    createdAt: new Date("2023-08-04"),
    updatedAt: new Date("2023-08-04"),
    ...overrides,
  };
};
