/* eslint-disable @typescript-eslint/no-explicit-any */

import { ILogger } from "@consensys/linea-shared-utils";

import {
  TEST_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "./constants";
import { Message, MessageProps } from "../../core/entities/Message";
import { Direction } from "../../core/enums";
import { MessageStatus } from "../../core/enums";
import { TransactionReceipt, TransactionSubmission } from "../../core/types";
import { MessageEntity } from "../../infrastructure/persistence/entities/Message.entity";

export class TestLogger implements ILogger {
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
}

export const generateReceipt = (overrides: Partial<TransactionReceipt> = {}): TransactionReceipt => ({
  hash: TEST_TRANSACTION_HASH,
  blockNumber: 200,
  status: "success",
  gasUsed: 50_000n,
  gasPrice: 100_000_000_000n,
  logs: [],
  ...overrides,
});

export const generateSubmission = (overrides: Partial<TransactionSubmission> = {}): TransactionSubmission => ({
  hash: TEST_TRANSACTION_HASH,
  nonce: 42,
  gasLimit: 60_000n,
  maxFeePerGas: 200_000_000_000n,
  maxPriorityFeePerGas: 2_000_000_000n,
  ...overrides,
});

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
    claimCycleCount: 0,
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
    claimCycleCount: 0,
    isForSponsorship: false,
    createdAt: new Date("2023-08-04"),
    updatedAt: new Date("2023-08-04"),
    ...overrides,
  };
};

/**
 * Temporarily sets environment variables for the duration of a callback,
 * then restores the original values (including deleting vars that weren't set before).
 */
export async function withEnv(vars: Record<string, string>, fn: () => Promise<void> | void): Promise<void> {
  const originals: Record<string, string | undefined> = {};
  for (const key of Object.keys(vars)) {
    originals[key] = process.env[key];
    process.env[key] = vars[key];
  }
  try {
    await fn();
  } finally {
    for (const key of Object.keys(originals)) {
      if (originals[key] === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = originals[key];
      }
    }
  }
}
