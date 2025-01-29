import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { TestEfficientLeftRightKeccak } from "../../../typechain-types";
import { deployFromFactory } from "../common/deployment";
import { generateKeccak256, generateRandomBytes } from "../common/helpers";

describe("EfficientLeftRightKeccak Library", () => {
  let contract: TestEfficientLeftRightKeccak;

  async function deployTestEfficientLeftRightKeccakFixture() {
    return deployFromFactory("TestEfficientLeftRightKeccak");
  }
  beforeEach(async () => {
    contract = (await loadFixture(deployTestEfficientLeftRightKeccakFixture)) as TestEfficientLeftRightKeccak;
  });

  describe("efficientKeccak", () => {
    it("Should return the correct keccak hash", async () => {
      const leftValue = generateRandomBytes(32);
      const rightValue = generateRandomBytes(32);
      const solidityKeccakHash = generateKeccak256(["bytes32", "bytes32"], [leftValue, rightValue]);
      expect(await contract.efficientKeccak(leftValue, rightValue)).to.equal(solidityKeccakHash);
    });
  });
});
