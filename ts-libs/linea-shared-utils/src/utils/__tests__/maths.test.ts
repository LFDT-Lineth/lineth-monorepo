import { safeSub, min } from "../maths";

describe("safeSub", () => {
  it("returns the difference when the minuend is greater than the subtrahend", () => {
    expect(safeSub(10n, 3n)).toBe(7n);
  });

  it("returns zero when values are equal", () => {
    expect(safeSub(5n, 5n)).toBe(0n);
  });

  it("returns zero when the minuend is smaller than the subtrahend", () => {
    expect(safeSub(2n, 8n)).toBe(0n);
  });
});

describe("min", () => {
  it("returns the first value when it is smaller", () => {
    expect(min(1n, 4n)).toBe(1n);
  });

  it("returns the second value when it is smaller", () => {
    expect(min(9n, 3n)).toBe(3n);
  });

  it("returns either value when they are equal", () => {
    expect(min(6n, 6n)).toBe(6n);
  });
});
