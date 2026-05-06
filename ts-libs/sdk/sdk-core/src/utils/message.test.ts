import { formatMessageStatus } from "./message";
import { OnChainMessageStatus } from "../types/message";

describe("message", () => {
  describe("formatMessageStatus", () => {
    it("should return CLAIMED for status 2n", () => {
      expect(formatMessageStatus(2n)).toBe(OnChainMessageStatus.CLAIMED);
    });

    it("should return CLAIMABLE for status 1n", () => {
      expect(formatMessageStatus(1n)).toBe(OnChainMessageStatus.CLAIMABLE);
    });

    it("should return UNKNOWN for status 0n", () => {
      expect(formatMessageStatus(0n)).toBe(OnChainMessageStatus.UNKNOWN);
    });

    it.each([3n, 100n, 999n])("should return UNKNOWN for unrecognized status %s", (status) => {
      expect(formatMessageStatus(status)).toBe(OnChainMessageStatus.UNKNOWN);
    });
  });
});
