import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";

import { deployFromFactory } from "../common/deployment.js";
import { generateKeccak256, generateRandomBytes } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("EfficientLeftRightKeccak Library", () => {
  let contract: ContractReturnType<"TestEfficientLeftRightKeccak">;

  async function deployTestEfficientLeftRightKeccakFixture() {
    return deployFromFactory("TestEfficientLeftRightKeccak");
  }
  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    contract = await networkHelpers.loadFixture(deployTestEfficientLeftRightKeccakFixture);
  });

  describe("efficientKeccak", () => {
    it("Should return the correct keccak hash", async () => {
      const leftValue = generateRandomBytes(32);
      const rightValue = generateRandomBytes(32);
      const solidityKeccakHash = generateKeccak256(["bytes32", "bytes32"], [leftValue, rightValue]);
      expect(await contract.read.efficientKeccak([leftValue, rightValue])).to.equal(solidityKeccakHash);
    });
  });
});
