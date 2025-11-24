import { bigintReplacer, isString, serialize } from "../string";

describe("bigintReplacer", () => {
  it("converts bigint values to strings", () => {
    expect(bigintReplacer("amount", 123n)).toBe("123");
    expect(bigintReplacer("negative", -456n)).toBe("-456");
  });

  it("returns non-bigint values unchanged", () => {
    const value = { foo: "bar" };
    expect(bigintReplacer("object", value)).toBe(value);
    expect(bigintReplacer("number", 10)).toBe(10);
  });
});

describe("serialize", () => {
  it("stringifies bigint values recursively", () => {
    const input = {
      total: 999n,
      nested: {
        arr: [1n, 2, { inner: 3n }],
      },
    };

    const serialized = serialize(input);
    expect(serialized).toBe('{"total":"999","nested":{"arr":["1",2,{"inner":"3"}]}}');
  });

  it("matches JSON.stringify behaviour for non-bigint values", () => {
    const input = { foo: "bar", count: 3 };
    expect(serialize(input)).toBe(JSON.stringify(input));
  });
});

describe("isString", () => {
  it("returns true for primitive strings", () => {
    expect(isString("hello")).toBe(true);
  });

  it("returns false for non-strings", () => {
    expect(isString(123)).toBe(false);
    expect(isString({ text: "hi" })).toBe(false);
    expect(isString(new String("wrapped"))).toBe(false);
  });
});
