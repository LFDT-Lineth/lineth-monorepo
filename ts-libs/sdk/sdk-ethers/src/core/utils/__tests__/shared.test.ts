import { describe, it, expect } from "@jest/globals";

import { isEmptyBytes, isNull, isString, isUndefined, subtractSeconds } from "../shared";

describe("Shared utils", () => {
  describe("subtractSeconds", () => {
    it("should substract X seconds to the current date", () => {
      const currentDate = new Date("2024-04-08T00:12:10.000Z");
      expect(subtractSeconds(currentDate, 10)).toStrictEqual(new Date("2024-04-08T00:12:00.000Z"));
    });
  });

  describe("isEmptyBytes", () => {
    it.each([
      {
        input: "0x",
        expectedValue: true,
      },
      {
        input: "",
        expectedValue: true,
      },
      {
        input: "0",
        expectedValue: true,
      },
      {
        input: "0x01",
        expectedValue: false,
      },
      {
        input: "0x0101",
        expectedValue: false,
      },
    ])("should return $expectedValue when bytes = $input", ({ input, expectedValue }) => {
      expect(isEmptyBytes(input)).toEqual(expectedValue);
    });
  });

  describe("isString", () => {
    it.each([
      {
        input: "",
        expectedValue: true,
      },
      {
        input: "test string",
        expectedValue: true,
      },
      {
        input: 1,
        expectedValue: false,
      },
      {
        input: [],
        expectedValue: false,
      },
      {
        input: ["test string"],
        expectedValue: false,
      },
    ])("should return $expectedValue when input = $input", ({ input, expectedValue }) => {
      expect(isString(input)).toEqual(expectedValue);
    });
  });

  describe("isUndefined", () => {
    it.each([
      {
        input: undefined,
        expectedValue: true,
      },
      {
        input: null,
        expectedValue: false,
      },
      {
        input: 1,
        expectedValue: false,
      },
      {
        input: [],
        expectedValue: false,
      },
      {
        input: "test string",
        expectedValue: false,
      },
    ])("should return $expectedValue when input = $input", ({ input, expectedValue }) => {
      expect(isUndefined(input)).toEqual(expectedValue);
    });
  });

  describe("isNull", () => {
    it.each([
      {
        input: null,
        expectedValue: true,
      },
      {
        input: undefined,
        expectedValue: false,
      },
      {
        input: 1,
        expectedValue: false,
      },
      {
        input: [],
        expectedValue: false,
      },
      {
        input: "test string",
        expectedValue: false,
      },
    ])("should return $expectedValue when input = $input", ({ input, expectedValue }) => {
      expect(isNull(input)).toEqual(expectedValue);
    });
  });
});
