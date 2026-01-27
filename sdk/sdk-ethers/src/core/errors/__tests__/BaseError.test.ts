import { describe, it } from "@jest/globals";

import { makeBaseError } from "../utils";

describe("BaseError", () => {
  it("Should log error message when we only pass a short message", () => {
    expect(makeBaseError("An error message.").message).toStrictEqual("An error message.");
  });
});
