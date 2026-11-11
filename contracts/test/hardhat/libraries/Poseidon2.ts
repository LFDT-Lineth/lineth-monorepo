import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";

import poseidon2TestData from "../_testData/poseidon2-test-data.json" with { type: "json" };
import { deployFromFactory } from "../common/deployment.js";
import { hex, expectRevertWithCustomError } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Poseidon2", () => {
  let poseidon2: ContractReturnType<"Poseidon2">;

  async function deployPoseidon2Fixture() {
    return deployFromFactory("Poseidon2");
  }

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    poseidon2 = await networkHelpers.loadFixture(deployPoseidon2Fixture);
  });

  describe("hash", () => {
    it("Should return poseidon2 hash for each test case", async () => {
      for (const element of poseidon2TestData) {
        expect(await poseidon2.read.hash([hex(element.in)])).to.equal(element.out);
      }
    });

    it("Should revert if the data is zero length", async () => {
      await expectRevertWithCustomError(poseidon2, poseidon2.read.hash(["0x"]), "DataIsEmpty");
    });

    it("Should revert if the data is less than 32 and not mod32", async () => {
      await expectRevertWithCustomError(poseidon2, poseidon2.read.hash(["0x12"]), "DataIsNotMod32");
    });

    it("Should revert if the data is greater than 32 and not mod32", async () => {
      await expectRevertWithCustomError(
        poseidon2,
        poseidon2.read.hash(["0x103adbc490c2067eac112873462707eb2072813005a4ac3ab182135be336742423456789"]),
        "DataIsNotMod32",
      );
    });
  });

  describe("padBytes32", () => {
    it("Should pad bytes32 to 64 bytes", async () => {
      const data = [
        {
          input: "0xaaaabbbbaaaabbbbaaaabbbbaaaabbbbaaaabbbbaaaabbbbaaaabbbbaaaabbbb",
          output:
            "0x0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb",
        },
        {
          input: "0x419999040c43d42953d000a00c7d8cfd5b6cacef32aaf9e615456ae404fcf8b6",
          output:
            "0x000041990000990400000c430000d429000053d0000000a000000c7d00008cfd00005b6c0000acef000032aa0000f9e60000154500006ae4000004fc0000f8b6",
        },
      ];

      for (const { input, output } of data) {
        expect(await poseidon2.read.padBytes32([hex(input)])).to.deep.equal(output);
      }
    });
  });
});
