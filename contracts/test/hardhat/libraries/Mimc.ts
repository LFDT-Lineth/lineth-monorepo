import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { Mimc } from "../../../typechain-types";
import mimcTestData from "../_testData/mimc-test-data.json";
import { deployFromFactory } from "../common/deployment";
import { expectRevertWithCustomError } from "../common/helpers";

describe("Mimc", () => {
  let mimc: Mimc;

  async function deployMIMCFixture() {
    return deployFromFactory("Mimc") as Promise<Mimc>;
  }

  beforeEach(async () => {
    mimc = await loadFixture(deployMIMCFixture);
  });

  describe("hash", () => {
    it("Should return mimc hash for each test case", async () => {
      for (const element of mimcTestData) {
        const msgs = ethers.concat(element.in);
        expect(await mimc.hash(msgs)).to.equal(element.out);
      }
    });

    it("Should revert if the data is zero length", async () => {
      await expectRevertWithCustomError(mimc, mimc.hash("0x"), "DataMissing");
    });

    it("Should revert if the data is less than 32 and not mod32", async () => {
      await expectRevertWithCustomError(mimc, mimc.hash("0x12"), "DataIsNotMod32");
    });

    it("Should revert if the data is greater than 32 and not mod32", async () => {
      await expectRevertWithCustomError(
        mimc,
        mimc.hash("0x103adbc490c2067eac112873462707eb2072813005a4ac3ab182135be336742423456789"),
        "DataIsNotMod32",
      );
    });
  });
});
