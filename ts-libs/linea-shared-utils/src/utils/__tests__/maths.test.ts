import { absDiff, min, safeSub } from "../maths";

describe("safeSub", () => {
  it("returns difference when minuend is greater than subtrahend", () => {
    // Arrange
    const minuend = 10n;
    const subtrahend = 3n;
    const expected = 7n;

    // Act
    const result = safeSub(minuend, subtrahend);

    // Assert
    expect(result).toBe(expected);
  });

  it("returns zero when values are equal", () => {
    // Arrange
    const value = 5n;

    // Act
    const result = safeSub(value, value);

    // Assert
    expect(result).toBe(0n);
  });

  it("returns zero when minuend is smaller than subtrahend", () => {
    // Arrange
    const minuend = 2n;
    const subtrahend = 8n;

    // Act
    const result = safeSub(minuend, subtrahend);

    // Assert
    expect(result).toBe(0n);
  });
});

describe("min", () => {
  it("returns first value when it is smaller", () => {
    // Arrange
    const smaller = 1n;
    const larger = 4n;

    // Act
    const result = min(smaller, larger);

    // Assert
    expect(result).toBe(smaller);
  });

  it("returns second value when it is smaller", () => {
    // Arrange
    const larger = 9n;
    const smaller = 3n;

    // Act
    const result = min(larger, smaller);

    // Assert
    expect(result).toBe(smaller);
  });

  it("returns either value when they are equal", () => {
    // Arrange
    const value = 6n;

    // Act
    const result = min(value, value);

    // Assert
    expect(result).toBe(value);
  });
});

describe("absDiff", () => {
  it("returns difference when first value is greater", () => {
    // Arrange
    const larger = 10n;
    const smaller = 3n;
    const expectedDiff = 7n;

    // Act
    const result = absDiff(larger, smaller);

    // Assert
    expect(result).toBe(expectedDiff);
  });

  it("returns difference when second value is greater", () => {
    // Arrange
    const smaller = 3n;
    const larger = 10n;
    const expectedDiff = 7n;

    // Act
    const result = absDiff(smaller, larger);

    // Assert
    expect(result).toBe(expectedDiff);
  });

  it("returns zero when values are equal", () => {
    // Arrange
    const value = 5n;

    // Act
    const result = absDiff(value, value);

    // Assert
    expect(result).toBe(0n);
  });

  it("returns zero when both values are zero", () => {
    // Arrange
    const zero = 0n;

    // Act
    const result = absDiff(zero, zero);

    // Assert
    expect(result).toBe(0n);
  });

  it("computes difference for large bigint values", () => {
    // Arrange
    const largeValue = 1000000000000000000n;
    const mediumValue = 500000000000000000n;
    const expectedDiff = 500000000000000000n;

    // Act
    const result = absDiff(largeValue, mediumValue);

    // Assert
    expect(result).toBe(expectedDiff);
  });
});
