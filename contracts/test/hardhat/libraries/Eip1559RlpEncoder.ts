import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";

import { TestEip1559RlpEncoder } from "../../../typechain-types";
import transactionWithCalldata from "../_testData/eip1559RlpEncoderTransactions/withCalldata.json";
import transactionWithCalldataAndAccessList from "../_testData/eip1559RlpEncoderTransactions/withCalldataAndAccessList.json";
import transactionWithLargeCalldata from "../_testData/eip1559RlpEncoderTransactions/withLargeCalldata.json";
import transactionWithoutCalldata from "../_testData/eip1559RlpEncoderTransactions/withoutCalldata.json";
import { deployFromFactory } from "../common/deployment";
import { generateKeccak256BytesDirectly } from "../common/helpers";
import { buildEip1559Transaction } from "../common/helpers/typedTransactionBuilding";

describe("Eip1559RlpEncoder Library", () => {
  let contract: TestEip1559RlpEncoder;

  async function deployTestEip1559RlpEncoderFixture() {
    return deployFromFactory("TestEip1559RlpEncoder", 1);
  }
  beforeEach(async () => {
    contract = (await loadFixture(deployTestEip1559RlpEncoderFixture)) as TestEip1559RlpEncoder;
  });

  describe("RLP Encoding and hashing", () => {
    it("Succeeds for a transaction without calldata", async () => {
      const { rlpEncodedTransaction, transactionHash } = await contract.encodeEip1559Transaction(
        buildEip1559Transaction(transactionWithoutCalldata.result),
      );

      expect(transactionWithoutCalldata.result.hash).equal(transactionHash);
      expect(transactionWithoutCalldata.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(transactionWithoutCalldata.result.hash);
    });

    it("Succeeds for a transaction with calldata", async () => {
      const { rlpEncodedTransaction, transactionHash } = await contract.encodeEip1559Transaction(
        buildEip1559Transaction(transactionWithCalldata.result),
      );

      expect(transactionWithCalldata.result.hash).equal(transactionHash);
      expect(transactionWithCalldata.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(transactionWithCalldata.result.hash);
    });

    it("Succeeds for a transaction with large calldata", async () => {
      const { rlpEncodedTransaction, transactionHash } = await contract.encodeEip1559Transaction(
        buildEip1559Transaction(transactionWithLargeCalldata.result),
      );

      expect(transactionWithLargeCalldata.result.hash).equal(transactionHash);
      expect(transactionWithLargeCalldata.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(transactionWithLargeCalldata.result.hash);
    });

    it("Succeeds for a transaction with calldata and an access list", async () => {
      const sepoliaContract = (await deployFromFactory("TestEip1559RlpEncoder", 11155111)) as TestEip1559RlpEncoder;

      const { rlpEncodedTransaction, transactionHash } = await sepoliaContract.encodeEip1559Transaction(
        buildEip1559Transaction(transactionWithCalldataAndAccessList.result),
      );

      expect(transactionWithCalldataAndAccessList.result.hash).equal(transactionHash);
      expect(transactionWithCalldataAndAccessList.rlpEncoded).equal(rlpEncodedTransaction);
      expect(generateKeccak256BytesDirectly(rlpEncodedTransaction)).equal(
        transactionWithCalldataAndAccessList.result.hash,
      );
    });
  });
});
