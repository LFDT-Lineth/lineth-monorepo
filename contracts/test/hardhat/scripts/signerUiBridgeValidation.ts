import { expect } from "chai";

import { calldataMatchesPromptForSignerUiValidation } from "../../../scripts/hardhat/signer-ui-bridge";

describe("signer-ui bridge calldata validation", () => {
  it("accepts exact raw calldata even when bytes look like ASCII hex characters", () => {
    const expectedAndActual = "0x31323334";
    expect(calldataMatchesPromptForSignerUiValidation(expectedAndActual, expectedAndActual)).to.equal(true);
  });

  it("accepts ASCII-hex wrapped calldata from RPC when decoded payload matches expected", () => {
    // "1234" (ascii bytes) represented as hex bytes: 0x31 32 33 34
    const asciiWrapped = "0x31323334";
    const expectedDecoded = "0x1234";
    expect(calldataMatchesPromptForSignerUiValidation(expectedDecoded, asciiWrapped)).to.equal(true);
  });

  it("rejects calldata mismatch when neither raw nor ASCII-decoded payload matches expected", () => {
    expect(calldataMatchesPromptForSignerUiValidation("0x1234", "0xabcd")).to.equal(false);
  });

  it("accepts regular non-wrapped calldata when it matches expected", () => {
    expect(calldataMatchesPromptForSignerUiValidation("0xdeadbeef", "0xdeadbeef")).to.equal(true);
  });

  it("accepts ASCII-hex wrapped calldata with uppercase ASCII bytes", () => {
    // "ABCD" as ASCII bytes => 0x41 42 43 44, decodes to 0xabcd
    expect(calldataMatchesPromptForSignerUiValidation("0xabcd", "0x41424344")).to.equal(true);
  });

  it("rejects malformed ASCII wrapper bytes that are not hex digits", () => {
    // "123g" as ASCII bytes => last byte 0x67 ("g"), not a hex digit
    expect(calldataMatchesPromptForSignerUiValidation("0x1234", "0x31323367")).to.equal(false);
  });
});
