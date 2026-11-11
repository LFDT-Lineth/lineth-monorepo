import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";

import transactionWithCalldata from "../_testData/eip1559RlpEncoderTransactions/withCalldata.json" with { type: "json" };
import transactionWithCalldataAndAccessList from "../_testData/eip1559RlpEncoderTransactions/withCalldataAndAccessList.json" with { type: "json" };
import transactionWithLargeCalldata from "../_testData/eip1559RlpEncoderTransactions/withLargeCalldata.json" with { type: "json" };
import transactionWithoutCalldata from "../_testData/eip1559RlpEncoderTransactions/withoutCalldata.json" with { type: "json" };
import { deployFromFactory } from "../common/deployment.js";
import { generateKeccak256BytesDirectly } from "../common/helpers/index.js";
import { buildEip1559Transaction } from "../common/helpers/typedTransactionBuilding.js";

import type { Eip1559Transaction } from "../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

// The struct's `yParity` is a `uint8`, which viem encodes from a `number`.
const toAbiTransaction = (transaction: Eip1559Transaction) => ({
  ...transaction,
  yParity: Number(transaction.yParity),
});

describe("Eip1559RlpEncoder Library", () => {
  let contract: ContractReturnType<"TestEip1559RlpEncoder">;

  async function deployTestEip1559RlpEncoderFixture() {
    return deployFromFactory("TestEip1559RlpEncoder", [1n]);
  }
  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    contract = await networkHelpers.loadFixture(deployTestEip1559RlpEncoderFixture);
  });

  describe("RLP Encoding and hashing", () => {
    it("Succeeds for a transaction without calldata", async () => {
      const [rlpEncodedTransaction, transactionHash] = await contract.read.encodeEip1559Transaction([
        toAbiTransaction(buildEip1559Transaction(transactionWithoutCalldata.result)),
      ]);

      expect(transactionWithoutCalldata.result.hash).equal(transactionHash);
      expect(transactionWithoutCalldata.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(transactionWithoutCalldata.result.hash);
    });

    it("Succeeds for a transaction with calldata", async () => {
      const [rlpEncodedTransaction, transactionHash] = await contract.read.encodeEip1559Transaction([
        toAbiTransaction(buildEip1559Transaction(transactionWithCalldata.result)),
      ]);

      expect(transactionWithCalldata.result.hash).equal(transactionHash);
      expect(transactionWithCalldata.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(transactionWithCalldata.result.hash);
    });

    it("Succeeds for a transaction with large calldata", async () => {
      const [rlpEncodedTransaction, transactionHash] = await contract.read.encodeEip1559Transaction([
        toAbiTransaction(buildEip1559Transaction(transactionWithLargeCalldata.result)),
      ]);

      expect(transactionWithLargeCalldata.result.hash).equal(transactionHash);
      expect(transactionWithLargeCalldata.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(transactionWithLargeCalldata.result.hash);
    });

    it("Succeeds for a transaction with calldata and an access list", async () => {
      const sepoliaContract = await deployFromFactory("TestEip1559RlpEncoder", [11155111n]);

      const [rlpEncodedTransaction, transactionHash] = await sepoliaContract.read.encodeEip1559Transaction([
        toAbiTransaction(buildEip1559Transaction(transactionWithCalldataAndAccessList.result)),
      ]);

      expect(transactionWithCalldataAndAccessList.result.hash).equal(transactionHash);
      expect(transactionWithCalldataAndAccessList.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(
        transactionWithCalldataAndAccessList.result.hash,
      );
    });
  });
});
