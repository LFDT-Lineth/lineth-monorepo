import { describe, it, beforeEach, expect } from "@jest/globals";
import { parseEther, serializeTransaction, toBytes, toRlp } from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { GoNativeCompressor } from "../GoNativeCompressor";

const TEST_ADDRESS = "0x0000000000000000000000000000000000000001";
const TEST_PRIVATE_KEY = "0x0000000000000000000000000000000000000000000000000000000000000001";

describe("GoNativeCompressor", () => {
  const dataLimit = 800_000;
  let compressor: GoNativeCompressor;

  beforeEach(() => {
    compressor = new GoNativeCompressor(dataLimit);
  });

  describe("getCompressedTxSize", () => {
    it("Should throw an error if an error occured during tx compression", () => {
      const transaction = serializeTransaction({
        to: TEST_ADDRESS,
        value: parseEther("2"),
        maxFeePerGas: parseEther("0.5"),
        maxPriorityFeePerGas: parseEther("0.45"),
        nonce: 1,
        chainId: 1,
      });
      const rlpEncodedTransaction = toRlp(transaction);
      const input = toBytes(rlpEncodedTransaction);

      expect(() => compressor.getCompressedTxSize(input)).toThrow(
        "Error while compressing the transaction: rlp: too few elements for types.DynamicFeeTx",
      );
    });

    it("Should return compressed tx size", async () => {
      const signer = privateKeyToAccount(TEST_PRIVATE_KEY);
      const encodedSignedTx = await signer.signTransaction({
        to: TEST_ADDRESS,
        value: parseEther("2"),
        maxFeePerGas: parseEther("0.5"),
        maxPriorityFeePerGas: parseEther("0.45"),
        nonce: 1,
        chainId: 1,
      });

      const rlpEncodedTransaction = toRlp(encodedSignedTx);
      const input = toBytes(rlpEncodedTransaction);

      const compressedTxSize = compressor.getCompressedTxSize(input);

      expect(compressedTxSize).toStrictEqual(63);
    });
  });
});
