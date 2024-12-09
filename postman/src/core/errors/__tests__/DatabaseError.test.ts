import { describe, it, expect } from "@jest/globals";
import { Direction, serialize } from "@consensys/linea-sdk";
import { DatabaseAccessError } from "../DatabaseErrors";
import { DatabaseErrorType, DatabaseRepoName, MessageStatus } from "../../enums";
import { MessageProps } from "../../entities/Message";
import { ZERO_ADDRESS, ZERO_HASH } from "../../constants";

describe("DatabaseAccessError", () => {
  it("Should log error message and reason when we pass a short message and the error", () => {
    const error = new DatabaseAccessError(
      DatabaseRepoName.MessageRepository,
      DatabaseErrorType.Read,
      new Error("read database error."),
    );
    expect(serialize(error)).toStrictEqual(
      serialize({
        name: "DatabaseAccessError",
        message: "MessageRepository: read - read database error.",
      }),
    );
  });

  it("Should log full error when we pass a short message, the error and the rejectedMessage", () => {
    const rejectedMessage: MessageProps = {
      messageHash: ZERO_HASH,
      messageSender: ZERO_ADDRESS,
      destination: ZERO_ADDRESS,
      fee: 0n,
      value: 0n,
      messageNonce: 0n,
      calldata: "0x",
      contractAddress: ZERO_ADDRESS,
      sentBlockNumber: 0,
      direction: Direction.L1_TO_L2,
      status: MessageStatus.SENT,
      claimNumberOfRetry: 0,
    };
    const error = new DatabaseAccessError(
      DatabaseRepoName.MessageRepository,
      DatabaseErrorType.Read,
      new Error("read database error."),
      rejectedMessage,
    );

    expect(serialize(error)).toStrictEqual(
      serialize({
        name: "DatabaseAccessError",
        message: "MessageRepository: read - read database error.",
        rejectedMessage: {
          messageHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
          messageSender: "0x0000000000000000000000000000000000000000",
          destination: "0x0000000000000000000000000000000000000000",
          fee: 0n,
          value: 0n,
          messageNonce: 0n,
          calldata: "0x",
          contractAddress: "0x0000000000000000000000000000000000000000",
          sentBlockNumber: 0,
          direction: "L1_TO_L2",
          status: "SENT",
          claimNumberOfRetry: 0,
        },
      }),
    );
  });
});
