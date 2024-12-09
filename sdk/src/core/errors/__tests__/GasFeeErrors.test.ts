import { describe, it } from "@jest/globals";
import { FeeEstimationError, GasEstimationError } from "../GasFeeErrors";
import { serialize } from "../../utils/serialize";
import { Message } from "../../types/message";
import { ZERO_ADDRESS, ZERO_HASH } from "../../constants";

describe("BaseError", () => {
  describe("FeeEstimationError", () => {
    it("Should log error message", () => {
      expect(serialize(new FeeEstimationError("An error message."))).toStrictEqual(
        serialize({
          name: "FeeEstimationError",
          message: "An error message.",
        }),
      );
    });
  });

  describe("GasEstimationError", () => {
    it("Should log error message", () => {
      const rejectedMessage: Message = {
        messageHash: ZERO_HASH,
        messageSender: ZERO_ADDRESS,
        destination: ZERO_ADDRESS,
        fee: 0n,
        value: 0n,
        messageNonce: 0n,
        calldata: "0x",
      };

      const estimationError = new Error("estimation error");

      expect(serialize(new GasEstimationError(estimationError.message, rejectedMessage))).toStrictEqual(
        serialize({
          name: "GasEstimationError",
          message: "estimation error",
          rejectedMessage: {
            messageHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
            messageSender: "0x0000000000000000000000000000000000000000",
            destination: "0x0000000000000000000000000000000000000000",
            fee: 0n,
            value: 0n,
            messageNonce: 0n,
            calldata: "0x",
          },
        }),
      );
    });
  });
});
