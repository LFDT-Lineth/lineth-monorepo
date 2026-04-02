import { IContractSignerClient } from "@consensys/linea-shared-utils";
import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { UnsupportedOperationError } from "../../../../../core/errors";
import { TEST_SIGNER_ADDRESS } from "../../../../../utils/testing/constants";
import { contractSignerToViemAccount } from "../contractSignerToViemAccount";

describe("contractSignerToViemAccount", () => {
  let signerClient: ReturnType<typeof mock<IContractSignerClient>>;

  beforeEach(() => {
    signerClient = mock<IContractSignerClient>();
    signerClient.getAddress.mockReturnValue(TEST_SIGNER_ADDRESS);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  it("returns an account with the correct address", () => {
    const account = contractSignerToViemAccount(signerClient);
    expect(account.address).toBe(TEST_SIGNER_ADDRESS);
  });

  it("signTransaction delegates to IContractSignerClient.sign and re-serializes", async () => {
    const tx = {
      to: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" as `0x${string}`,
      value: 0n,
      gas: 21000n,
      maxFeePerGas: 2_000_000_000n,
      maxPriorityFeePerGas: 1_000_000_000n,
      type: "eip1559" as const,
      nonce: 0,
      chainId: 1,
    };

    const fakeSignatureHex =
      "0x" +
      "b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000" +
      "b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000" +
      "1b";
    signerClient.sign.mockResolvedValue(fakeSignatureHex as `0x${string}`);

    const account = contractSignerToViemAccount(signerClient);
    const result = await account.signTransaction(tx);

    expect(signerClient.sign).toHaveBeenCalledWith(tx);
    expect(result).toMatch(/^0x/);
    expect(typeof result).toBe("string");
  });

  it("signMessage throws UnsupportedOperationError with signer address", async () => {
    const account = contractSignerToViemAccount(signerClient);
    await expect(account.signMessage({ message: "hello" })).rejects.toThrow(UnsupportedOperationError);
    await expect(account.signMessage({ message: "hello" })).rejects.toThrow(/signMessage is not supported/);
    await expect(account.signMessage({ message: "hello" })).rejects.toThrow(new RegExp(TEST_SIGNER_ADDRESS));
  });

  it("signTypedData throws UnsupportedOperationError with signer address", async () => {
    const account = contractSignerToViemAccount(signerClient);
    const call = () =>
      account.signTypedData({ domain: {}, types: {}, primaryType: "EIP712Domain", message: undefined });
    await expect(call()).rejects.toThrow(UnsupportedOperationError);
    await expect(call()).rejects.toThrow(/signTypedData is not supported/);
    await expect(call()).rejects.toThrow(new RegExp(TEST_SIGNER_ADDRESS));
  });
});
