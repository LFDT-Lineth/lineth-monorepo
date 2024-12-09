import { describe, it } from "@jest/globals";
import { BaseError } from "../BaseError";
import { serialize } from "../../utils/serialize";

describe("BaseError", () => {
  it("Should log error message when we only pass a short message", () => {
    expect(serialize(new BaseError("An error message."))).toStrictEqual(
      serialize({
        name: "LineaSDKCoreError",
        message: "An error message.",
      }),
    );
  });
});
