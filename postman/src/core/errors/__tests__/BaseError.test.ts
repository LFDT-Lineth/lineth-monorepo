import { serialize } from "@consensys/linea-sdk";
import { describe, it } from "@jest/globals";

import { BaseError } from "../BaseError";

describe("BaseError", () => {
  it("Should log error message when we only pass a short message", () => {
    expect(serialize(new BaseError("An error message."))).toStrictEqual(
      serialize({
        name: "PostmanCoreError",
        message: "An error message.",
      }),
    );
  });
});
