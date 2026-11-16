import { expect } from "chai";
import { describe, it, beforeEach } from "node:test";

import { randomBytes32 } from "../../../common/helpers/encoding.js";
import { generateKeccak256 } from "../../../common/helpers/hashing.js";
import { deployFromFactory } from "../common/deployment.js";
import { loadFixture } from "../common/helpers/connection.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("EfficientLeftRightKeccak Library", () => {
  let contract: ContractReturnType<"TestEfficientLeftRightKeccak">;

  async function deployTestEfficientLeftRightKeccakFixture() {
    return deployFromFactory("TestEfficientLeftRightKeccak");
  }
  beforeEach(async () => {
    contract = await loadFixture(deployTestEfficientLeftRightKeccakFixture);
  });

  describe("efficientKeccak", () => {
    it("Should return the correct keccak hash", async () => {
      const leftValue = randomBytes32();
      const rightValue = randomBytes32();
      const solidityKeccakHash = generateKeccak256(["bytes32", "bytes32"], [leftValue, rightValue], {
        encodePacked: false,
      });
      expect(await contract.read.efficientKeccak([leftValue, rightValue])).to.equal(solidityKeccakHash);
    });
  });
});
