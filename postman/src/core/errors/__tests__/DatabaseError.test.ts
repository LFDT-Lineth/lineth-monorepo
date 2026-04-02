import { describe, it, expect } from "@jest/globals";

import { ZERO_ADDRESS, ZERO_HASH } from "../../constants";
import { MessageProps } from "../../entities/Message";
import { Direction } from "../../enums";
import { DatabaseErrorType, DatabaseRepoName, MessageStatus } from "../../enums";
import { DatabaseAccessError } from "../DatabaseErrors";

describe("DatabaseAccessError", () => {
  it("Should log error message and reason when we pass a short message and the error", () => {
    const cause = new Error("read database error.");
    const error = new DatabaseAccessError(DatabaseRepoName.MessageRepository, DatabaseErrorType.Read, cause);
    expect(error.name).toStrictEqual("DatabaseAccessError");
    expect(error.message).toStrictEqual("MessageRepository: read - read database error.");
    expect((error.cause as Error).message).toStrictEqual("read database error.");
  });

  it("Should capture the rejected entity and chain the cause", () => {
    const rejectedEntity: MessageProps = {
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
      claimCycleCount: 0,
    };
    const cause = new Error("read database error.");
    const error = new DatabaseAccessError(
      DatabaseRepoName.MessageRepository,
      DatabaseErrorType.Read,
      cause,
      rejectedEntity,
    );

    expect(error.name).toStrictEqual("DatabaseAccessError");
    expect(error.message).toStrictEqual("MessageRepository: read - read database error.");
    expect((error.cause as Error).message).toStrictEqual("read database error.");
    expect(error.rejectedEntity).toStrictEqual(rejectedEntity);
    expect(error.cause).toBe(cause);
  });
});
