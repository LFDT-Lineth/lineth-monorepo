import { describe, it, expect } from "@jest/globals";

import { subtractSeconds } from "../shared";

describe("Shared utils", () => {
  describe("subtractSeconds", () => {
    it("should substract X seconds to the current date", () => {
      const currentDate = new Date("2024-04-08T00:12:10.000Z");
      expect(subtractSeconds(currentDate, 10)).toStrictEqual(new Date("2024-04-08T00:12:00.000Z"));
    });
  });
});
