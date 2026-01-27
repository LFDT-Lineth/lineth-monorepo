import { describe, it, expect } from "@jest/globals";

import { OnChainMessageStatus } from "../../enums/message";
import { formatMessageStatus } from "../message";

describe("Message utils", () => {
  describe("formatMessageStatus", () => {
    it("should return 'UNKNOWN' when status != 1 and status != 2", () => {
      expect(formatMessageStatus(0n)).toStrictEqual(OnChainMessageStatus.UNKNOWN);
    });

    it("should return 'CLAIMABLE' when status = 1", () => {
      expect(formatMessageStatus(1n)).toStrictEqual(OnChainMessageStatus.CLAIMABLE);
    });

    it("should return 'CLAIMED' when status = 2", () => {
      expect(formatMessageStatus(2n)).toStrictEqual(OnChainMessageStatus.CLAIMED);
    });
  });
});
