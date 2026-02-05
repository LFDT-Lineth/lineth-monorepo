import { test, expect } from "@playwright/test";

test.describe("Example unit tests", () => {
  test("basic assertion works", () => {
    expect(1 + 1).toBe(2);
  });

  test("string comparison works", () => {
    expect("hello").toBe("hello");
  });
});
