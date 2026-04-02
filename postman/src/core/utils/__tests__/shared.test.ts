import { describe, it, expect } from "@jest/globals";

import { serialize, subtractSeconds } from "../shared";

describe("Shared utils", () => {
  describe("subtractSeconds", () => {
    it("should substract X seconds to the current date", () => {
      const currentDate = new Date("2024-04-08T00:12:10.000Z");
      expect(subtractSeconds(currentDate, 10)).toStrictEqual(new Date("2024-04-08T00:12:00.000Z"));
    });
  });

  describe("serialize", () => {
    it("should convert bigint values to strings", () => {
      expect(serialize({ amount: 123n })).toBe('{"amount":"123"}');
    });

    it("should keep regular values as-is", () => {
      const obj = { name: "test", count: 42, active: true, extra: null };
      expect(serialize(obj)).toBe(JSON.stringify(obj));
    });
  });
});
