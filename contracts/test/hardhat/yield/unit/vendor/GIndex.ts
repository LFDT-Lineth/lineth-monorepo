import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";
import { type Hex, hexToBigInt, maxUint256, toHex, zeroHash } from "viem";

import { deployFromFactory } from "../../../common/deployment.js";
import { generateRandomBytes, expectRevertWithCustomError } from "../../../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("GIndex", () => {
  let gIndex: ContractReturnType<"TestGIndex">;
  let ZERO: Hex;
  let ROOT: Hex;
  let MAX: Hex;

  beforeEach(async () => {
    async function deployTestGIndex() {
      return deployFromFactory("TestGIndex");
    }
    const { networkHelpers } = await network.getOrCreate();
    gIndex = await networkHelpers.loadFixture(deployTestGIndex);
    ZERO = await gIndex.read.wrap([zeroHash]);
    ROOT = await gIndex.read.wrap(["0x0000000000000000000000000000000000000000000000000000000000000100"]);
    MAX = await gIndex.read.wrap(["0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"]);
  });

  it("test_pack", async () => {
    const gI = await gIndex.read.pack([hexToBigInt("0x7b426f79504c6a8e9d31415b722f696e705c8a3d9f41"), 42]);

    expect(await gIndex.read.unwrap([gI])).to.equal(
      "0x0000000000000000007b426f79504c6a8e9d31415b722f696e705c8a3d9f412a",
      "Invalid gindex encoded",
    );

    expect(hexToBigInt(await gIndex.read.unwrap([MAX]))).to.equal(maxUint256, "Invalid gindex encoded");
  });

  it("test_isRootTrue", async () => {
    expect(await gIndex.read.isRoot([ROOT])).to.be.true;
  });

  it("test_isRootFalse", async () => {
    expect(await gIndex.read.isRoot([await gIndex.read.pack([0n, 0])])).to.be.false;
    expect(await gIndex.read.isRoot([await gIndex.read.pack([42n, 0])])).to.be.false;
    expect(await gIndex.read.isRoot([await gIndex.read.pack([42n, 4])])).to.be.false;
    expect(await gIndex.read.isRoot([await gIndex.read.pack([2048n, 4])])).to.be.false;

    const maxUint248 = BigInt(2) ** BigInt(248) - BigInt(1);
    expect(await gIndex.read.isRoot([await gIndex.read.pack([maxUint248, 255])])).to.be.false;
  });

  it("test_concat", async () => {
    expect(
      await gIndex.read.unwrap([
        await gIndex.read.concat([await gIndex.read.pack([2n, 99]), await gIndex.read.pack([3n, 99])]),
      ]),
    ).to.equal(await gIndex.read.unwrap([await gIndex.read.pack([5n, 99])]));
    expect(
      await gIndex.read.unwrap([
        await gIndex.read.concat([await gIndex.read.pack([31n, 99]), await gIndex.read.pack([3n, 99])]),
      ]),
    ).to.equal(await gIndex.read.unwrap([await gIndex.read.pack([63n, 99])]));
    expect(
      await gIndex.read.unwrap([
        await gIndex.read.concat([await gIndex.read.pack([31n, 99]), await gIndex.read.pack([6n, 99])]),
      ]),
    ).to.equal(await gIndex.read.unwrap([await gIndex.read.pack([126n, 99])]));

    expect(
      await gIndex.read.unwrap([
        await gIndex.read.concat([
          await gIndex.read.concat([
            await gIndex.read.concat([ROOT, await gIndex.read.pack([2n, 1])]),
            await gIndex.read.pack([5n, 1]),
          ]),
          await gIndex.read.pack([9n, 1]),
        ]),
      ]),
    ).to.equal(await gIndex.read.unwrap([await gIndex.read.pack([73n, 1])]));

    expect(
      await gIndex.read.unwrap([
        await gIndex.read.concat([
          await gIndex.read.concat([
            await gIndex.read.concat([ROOT, await gIndex.read.pack([2n, 9])]),
            await gIndex.read.pack([5n, 1]),
          ]),
          await gIndex.read.pack([9n, 4]),
        ]),
      ]),
    ).to.equal(await gIndex.read.unwrap([await gIndex.read.pack([73n, 4])]));

    expect(await gIndex.read.unwrap([await gIndex.read.concat([ROOT, MAX])])).to.equal(await gIndex.read.unwrap([MAX]));
  });

  it("test_concat_RevertsIfZeroGIndex", async () => {
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.concat([ZERO, await gIndex.read.pack([1024n, 1])]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.concat([await gIndex.read.pack([1024n, 1]), ZERO]),
      "IndexOutOfRange",
    );
  });

  it("test_concat_BigIndicesBorderCases", async () => {
    await gIndex.read.concat([await gIndex.read.pack([2n ** 9n, 0]), await gIndex.read.pack([2n ** 238n, 0])]);
    await gIndex.read.concat([await gIndex.read.pack([2n ** 47n, 0]), await gIndex.read.pack([2n ** 200n, 0])]);
    await gIndex.read.concat([await gIndex.read.pack([2n ** 199n, 0]), await gIndex.read.pack([2n ** 48n, 0])]);
  });

  it("test_concat_RevertsIfTooBigIndices", async () => {
    await expectRevertWithCustomError(gIndex, gIndex.read.concat([MAX, MAX]), "IndexOutOfRange");

    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.concat([await gIndex.read.pack([2n ** 48n, 0]), await gIndex.read.pack([2n ** 200n, 0])]),
      "IndexOutOfRange",
    );

    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.concat([await gIndex.read.pack([2n ** 200n, 0]), await gIndex.read.pack([2n ** 48n, 0])]),
      "IndexOutOfRange",
    );
  });

  it("testFuzz_concat_WithRoot", async () => {
    for (let i = 0; i < 10; i++) {
      const randomIndex = (BigInt(generateRandomBytes(30)) % BigInt(2) ** BigInt(240)) + BigInt(1);
      const randomGIndex = await gIndex.read.wrap([toHex(randomIndex, { size: 32 })]);

      expect(await gIndex.read.unwrap([await gIndex.read.concat([ROOT, randomGIndex])])).to.equal(
        await gIndex.read.unwrap([randomGIndex]),
        "`concat` with a root should return right-hand side value",
      );
    }
  });

  it("testFuzz_unpack", async () => {
    for (let i = 0; i < 20; i++) {
      const index = BigInt(generateRandomBytes(30)) % BigInt(2) ** BigInt(240);
      const pow = BigInt(generateRandomBytes(1)) % 256n;

      const packed = await gIndex.read.pack([index, Number(pow)]);

      expect(await gIndex.read.index([packed])).to.equal(index);
      expect(await gIndex.read.width([packed])).to.equal(2n ** pow);
    }
  });

  it("test_shr", async () => {
    let gI = await gIndex.read.pack([1024n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1024n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1025n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 15n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1039n, 4])]),
    );

    gI = await gIndex.read.pack([1031n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1031n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1032n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 8n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1039n, 4])]),
    );

    gI = await gIndex.read.pack([2049n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([2049n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([2050n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([gI, 14n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([2063n, 4])]),
    );
  });

  it("test_shr_AfterConcat", async () => {
    const gIParent = await gIndex.read.pack([5n, 4]);

    let gI = await gIndex.read.pack([1024n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5120n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5121n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 15n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5135n, 4])]),
    );

    gI = await gIndex.read.pack([1031n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5127n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5128n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 8n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5135n, 4])]),
    );

    gI = await gIndex.read.pack([2049n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([10241n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([10242n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shr([await gIndex.read.concat([gIParent, gI]), 14n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([10255n, 4])]),
    );
  });

  it("test_shr_OffTheWidth", async () => {
    await expectRevertWithCustomError(gIndex, gIndex.read.shr([ROOT, 1n]), "IndexOutOfRange");
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.pack([1024n, 4]), 16n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.pack([1031n, 4]), 9n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.pack([1023n, 4]), 1n]),
      "IndexOutOfRange",
    );
  });

  it("test_shr_OffTheWidth_AfterConcat", async () => {
    const gIParent = await gIndex.read.pack([154n, 4]);

    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.concat([gIParent, ROOT]), 1n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.concat([gIParent, await gIndex.read.pack([1024n, 4])]), 16n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.concat([gIParent, await gIndex.read.pack([1031n, 4])]), 9n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shr([await gIndex.read.concat([gIParent, await gIndex.read.pack([1023n, 4])]), 1n]),
      "IndexOutOfRange",
    );
  });

  it("test_shl", async () => {
    let gI = await gIndex.read.pack([1023n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1023n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1022n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 15n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1008n, 4])]),
    );

    gI = await gIndex.read.pack([1031n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1031n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1030n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 7n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([1024n, 4])]),
    );

    gI = await gIndex.read.pack([2063n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([2063n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([2062n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([gI, 15n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([2048n, 4])]),
    );
  });

  it("test_shl_AfterConcat", async () => {
    const gIParent = await gIndex.read.pack([5n, 4]);

    let gI = await gIndex.read.pack([1023n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([3071n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([3070n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 15n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([3056n, 4])]),
    );

    gI = await gIndex.read.pack([1031n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5127n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5126n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 7n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([5120n, 4])]),
    );

    gI = await gIndex.read.pack([2063n, 4]);
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 0n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([10255n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 1n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([10254n, 4])]),
    );
    expect(await gIndex.read.unwrap([await gIndex.read.shl([await gIndex.read.concat([gIParent, gI]), 15n])])).to.equal(
      await gIndex.read.unwrap([await gIndex.read.pack([10240n, 4])]),
    );
  });

  it("test_shl_OffTheWidth", async () => {
    await expectRevertWithCustomError(gIndex, gIndex.read.shl([ROOT, 1n]), "IndexOutOfRange");
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.pack([1024n, 4]), 1n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.pack([1031n, 4]), 9n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.pack([1023n, 4]), 16n]),
      "IndexOutOfRange",
    );
  });

  it("test_shl_OffTheWidth_AfterConcat", async () => {
    const gIParent = await gIndex.read.pack([154n, 4]);

    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.concat([gIParent, ROOT]), 1n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.concat([gIParent, await gIndex.read.pack([1024n, 4])]), 1n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.concat([gIParent, await gIndex.read.pack([1031n, 4])]), 9n]),
      "IndexOutOfRange",
    );
    await expectRevertWithCustomError(
      gIndex,
      gIndex.read.shl([await gIndex.read.concat([gIParent, await gIndex.read.pack([1023n, 4])]), 16n]),
      "IndexOutOfRange",
    );
  });

  it("test_fls", async () => {
    for (let i = 1; i < 255; i++) {
      expect(await gIndex.read.fls([(1n << BigInt(i)) - 1n])).to.equal(BigInt(i - 1));
      expect(await gIndex.read.fls([1n << BigInt(i)])).to.equal(BigInt(i));
      expect(await gIndex.read.fls([(1n << BigInt(i)) + 1n])).to.equal(BigInt(i));
    }

    expect(await gIndex.read.fls([3n])).to.equal(1n); // 0011
    expect(await gIndex.read.fls([7n])).to.equal(2n); // 0111
    expect(await gIndex.read.fls([10n])).to.equal(3n); // 1010
    expect(await gIndex.read.fls([300n])).to.equal(8n); // 0001 0010 1100
    expect(await gIndex.read.fls([0n])).to.equal(256n);
  });
});
