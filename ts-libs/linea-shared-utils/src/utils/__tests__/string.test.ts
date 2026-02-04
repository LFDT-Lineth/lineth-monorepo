import { bigintReplacer, isString, serialize } from "../string";

describe("bigintReplacer", () => {
  it("converts positive bigint to string", () => {
    // Arrange
    const key = "amount";
    const value = 123n;

    // Act
    const result = bigintReplacer(key, value);

    // Assert
    expect(result).toBe("123");
  });

  it("converts negative bigint to string", () => {
    // Arrange
    const key = "negative";
    const value = -456n;

    // Act
    const result = bigintReplacer(key, value);

    // Assert
    expect(result).toBe("-456");
  });

  it("returns object values unchanged", () => {
    // Arrange
    const key = "object";
    const value = { foo: "bar" };

    // Act
    const result = bigintReplacer(key, value);

    // Assert
    expect(result).toBe(value);
  });

  it("returns number values unchanged", () => {
    // Arrange
    const key = "number";
    const value = 10;

    // Act
    const result = bigintReplacer(key, value);

    // Assert
    expect(result).toBe(value);
  });
});

describe("serialize", () => {
  it("converts nested bigint values to strings", () => {
    // Arrange
    const input = {
      total: 999n,
      nested: {
        arr: [1n, 2, { inner: 3n }],
      },
    };

    // Act
    const result = serialize(input);

    // Assert
    expect(result).toBe('{"total":"999","nested":{"arr":["1",2,{"inner":"3"}]}}');
  });

  it("produces same output as JSON.stringify for non-bigint values", () => {
    // Arrange
    const input = { foo: "bar", count: 3 };

    // Act
    const result = serialize(input);

    // Assert
    expect(result).toBe(JSON.stringify(input));
  });
});

describe("isString", () => {
  it("returns true for primitive string", () => {
    // Arrange
    const value = "hello";

    // Act
    const result = isString(value);

    // Assert
    expect(result).toBe(true);
  });

  it("returns false for number", () => {
    // Arrange
    const value = 123;

    // Act
    const result = isString(value);

    // Assert
    expect(result).toBe(false);
  });

  it("returns false for object", () => {
    // Arrange
    const value = { text: "hi" };

    // Act
    const result = isString(value);

    // Assert
    expect(result).toBe(false);
  });

  it("returns false for String object", () => {
    // Arrange
    const value = new String("wrapped");

    // Act
    const result = isString(value);

    // Assert
    expect(result).toBe(false);
  });
});
