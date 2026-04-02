import { describe, it } from "@jest/globals";

import { BaseError } from "../BaseError";

describe("BaseError", () => {
  it("Should log error message when we only pass a short message", () => {
    const error = new BaseError("An error message.");
    expect(error.name).toStrictEqual("PostmanCoreError");
    expect(error.message).toStrictEqual("An error message.");
    expect(error.cause).toBeUndefined();
  });

  it("Should log error cause when we pass a cause to the BaseError", () => {
    const error = new BaseError("An error message.", { cause: new Error("Inner error") });
    expect(error.name).toStrictEqual("PostmanCoreError");
    expect(error.message).toStrictEqual("An error message.");
    expect((error.cause as Error).message).toStrictEqual("Inner error");
  });

  it("Should use fallback message when no message is provided", () => {
    const error = new BaseError();
    expect(error.name).toStrictEqual("PostmanCoreError");
    expect(error.message).toStrictEqual("An error occurred.");
  });

  it("Should use fallback message when an empty string is provided", () => {
    const error = new BaseError("");
    expect(error.name).toStrictEqual("PostmanCoreError");
    expect(error.message).toStrictEqual("An error occurred.");
  });
});
