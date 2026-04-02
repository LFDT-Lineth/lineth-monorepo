import { IContractSignerClient } from "@consensys/linea-shared-utils";
import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { bytesToHex } from "viem/utils";

import { LineaGasFees } from "../../../../../core/clients/blockchain/IGasProvider";
import { TransactionRequest } from "../../../../../core/types";
import { ViemTransactionSigner } from "../ViemTransactionSigner";

const TEST_TX: TransactionRequest = {
  from: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  to: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  data: "0x",
  value: 0n,
};

const TEST_FEES: LineaGasFees = {
  gasLimit: 21000n,
  maxFeePerGas: 2_000_000_000n,
  maxPriorityFeePerGas: 1_000_000_000n,
};

// 65-byte signature hex (r=32 bytes + s=32 bytes + v=1 byte)
const FAKE_SIGNATURE_HEX = ("0x" +
  "b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000" + // r
  "b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000" + // s
  "1b") as `0x${string}`; // v

describe("ViemTransactionSigner", () => {
  let signerClient: ReturnType<typeof mock<IContractSignerClient>>;
  let txSigner: ViemTransactionSigner;

  beforeEach(() => {
    signerClient = mock<IContractSignerClient>();
    txSigner = new ViemTransactionSigner(signerClient);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  it("calls IContractSignerClient.sign with the correct viem transaction shape", async () => {
    signerClient.sign.mockResolvedValue(FAKE_SIGNATURE_HEX);

    const result = await txSigner.signAndSerialize(TEST_TX, TEST_FEES);

    expect(signerClient.sign).toHaveBeenCalledTimes(1);
    expect(signerClient.sign).toHaveBeenCalledWith(
      expect.objectContaining({
        to: TEST_TX.to,
        gas: TEST_FEES.gasLimit,
        maxFeePerGas: TEST_FEES.maxFeePerGas,
        maxPriorityFeePerGas: TEST_FEES.maxPriorityFeePerGas,
        type: "eip1559",
      }),
    );
    expect(result).toBeInstanceOf(Uint8Array);
    expect(result.length).toBeGreaterThan(0);
  });

  it("defaults value to 0n when not provided", async () => {
    signerClient.sign.mockResolvedValue(FAKE_SIGNATURE_HEX);

    const txWithoutValue: TransactionRequest = { to: TEST_TX.to! };
    const result = await txSigner.signAndSerialize(txWithoutValue, TEST_FEES);

    expect(signerClient.sign).toHaveBeenCalledWith(expect.objectContaining({ value: 0n }));
    expect(result).toBeInstanceOf(Uint8Array);
  });

  it("uses the provided chainId when serializing", async () => {
    signerClient.sign.mockResolvedValue(FAKE_SIGNATURE_HEX);
    const signerWithChain = new ViemTransactionSigner(signerClient, 59144);

    await signerWithChain.signAndSerialize(TEST_TX, TEST_FEES);

    expect(signerClient.sign).toHaveBeenCalledWith(expect.objectContaining({ chainId: 59144 }));
  });

  it("produces a serialized output larger than the bare transaction", async () => {
    signerClient.sign.mockResolvedValue(FAKE_SIGNATURE_HEX);

    const result = await txSigner.signAndSerialize(TEST_TX, TEST_FEES);
    // A serialized EIP-1559 tx with signature should be at least 90 bytes
    expect(result.length).toBeGreaterThan(90);
  });

  it("produces the same bytes as viem serializeTransaction with the parsed signature", async () => {
    signerClient.sign.mockResolvedValue(FAKE_SIGNATURE_HEX);

    const result = await txSigner.signAndSerialize(TEST_TX, TEST_FEES);
    expect(bytesToHex(result)).toBe(
      "0xb86d02f86a0180843b9aca00847735940082520894bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb8080c080a0b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000a0b94f5374fce5edbc8e2a8697c15331677e6ebf0b000000000000000000000000",
    );
  });
});
