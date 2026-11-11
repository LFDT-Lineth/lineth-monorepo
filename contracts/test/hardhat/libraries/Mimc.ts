import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";
import { concat } from "viem";

import mimcTestData from "../_testData/mimc-test-data.json" with { type: "json" };
import { deployFromFactory } from "../common/deployment.js";
import { hex, expectRevertWithCustomError } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Mimc", () => {
  let mimc: ContractReturnType<"Mimc">;

  async function deployMIMCFixture() {
    return deployFromFactory("Mimc");
  }

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    mimc = await networkHelpers.loadFixture(deployMIMCFixture);
  });

  describe("hash", () => {
    it("Should return mimc hash for each test case", async () => {
      for (const element of mimcTestData) {
        const msgs = concat(element.in.map(hex));
        expect(await mimc.read.hash([msgs])).to.equal(element.out);
      }
    });

    it("Should revert if the data is zero length", async () => {
      await expectRevertWithCustomError(mimc, mimc.read.hash(["0x"]), "DataMissing");
    });

    it("Should revert if the data is less than 32 and not mod32", async () => {
      await expectRevertWithCustomError(mimc, mimc.read.hash(["0x12"]), "DataIsNotMod32");
    });

    it("Should revert if the data is greater than 32 and not mod32", async () => {
      await expectRevertWithCustomError(
        mimc,
        mimc.read.hash(["0x103adbc490c2067eac112873462707eb2072813005a4ac3ab182135be336742423456789"]),
        "DataIsNotMod32",
      );
    });
  });
});
