import { slice, hexToNumber } from "./misc";

import type { Hex } from "../types/misc";

const FOUR_BYTES_HEX = "0xaabbccdd" as Hex;
const TWO_BYTES_HEX = "0xaabb" as Hex;
const THIRTEEN_BYTES_HEX = "0x0102030405060708090a0b0c0d" as Hex;

describe("misc", () => {
  describe("slice", () => {
    it("should extract the first byte", () => {
      expect(slice(FOUR_BYTES_HEX, 0, 1)).toBe("0xaa");
    });

    it("should extract bytes from the middle", () => {
      expect(slice(FOUR_BYTES_HEX, 1, 3)).toBe("0xbbcc");
    });

    it("should extract the last byte", () => {
      expect(slice(FOUR_BYTES_HEX, 3, 4)).toBe("0xdd");
    });

    it("should return an empty hex when start equals end", () => {
      expect(slice(FOUR_BYTES_HEX, 2, 2)).toBe("0x");
    });

    it("should extract multiple non-overlapping ranges", () => {
      expect(slice(THIRTEEN_BYTES_HEX, 0, 4)).toBe("0x01020304");
      expect(slice(THIRTEEN_BYTES_HEX, 4, 8)).toBe("0x05060708");
    });

    it("should return a truncated result when end exceeds hex length", () => {
      expect(slice(TWO_BYTES_HEX, 0, 4)).toBe("0xaabb");
    });

    it("should return an empty hex when start is beyond hex length", () => {
      expect(slice(TWO_BYTES_HEX, 5, 6)).toBe("0x");
    });

    it("should return an empty hex when start exceeds end", () => {
      expect(slice(FOUR_BYTES_HEX, 3, 1)).toBe("0x");
    });
  });

  describe("hexToNumber", () => {
    it("should convert 0x0 to 0", () => {
      expect(hexToNumber("0x0" as Hex)).toBe(0);
    });

    it("should convert 0xff to 255", () => {
      expect(hexToNumber("0xff" as Hex)).toBe(255);
    });

    it("should convert 0x1 to 1", () => {
      expect(hexToNumber("0x1" as Hex)).toBe(1);
    });

    it("should convert a multi-byte hex to the correct number", () => {
      expect(hexToNumber("0x0100" as Hex)).toBe(256);
    });

    it("should handle uppercase hex digits", () => {
      expect(hexToNumber("0xFF" as Hex)).toBe(255);
    });
  });
});
