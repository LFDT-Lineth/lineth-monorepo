import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { TestGIndex } from "contracts/typechain-types";
import { deployFromFactory } from "../../../common/deployment";
import { hexlify, MaxUint256, randomBytes, toBeHex, ZeroHash, zeroPadValue } from "ethers";

describe("GIndex", () => {
  let gIndex: TestGIndex;
  let ZERO: string;
  let ROOT: string;
  let MAX: string;

  beforeEach(async () => {
    async function deployTestGIndex() {
      return deployFromFactory("TestGIndex");
    }
    gIndex = (await loadFixture(deployTestGIndex)) as TestGIndex;
    ZERO = await gIndex.wrap(ZeroHash);
    ROOT = await gIndex.wrap("0x0000000000000000000000000000000000000000000000000000000000000100");
    MAX = await gIndex.wrap("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff");
  });

  it("test_pack", async () => {
    const gI = await gIndex.pack("0x7b426f79504c6a8e9d31415b722f696e705c8a3d9f41", 42);

    expect(await gIndex.unwrap(gI)).to.equal(
      "0x0000000000000000007b426f79504c6a8e9d31415b722f696e705c8a3d9f412a",
      "Invalid gindex encoded",
    );

    expect(await gIndex.unwrap(MAX)).to.equal(MaxUint256, "Invalid gindex encoded");
  });

  it("test_isRootTrue", async () => {
    expect(await gIndex.isRoot(ROOT)).to.be.true;
  });

  it("test_isRootFalse", async () => {
    expect(await gIndex.isRoot(await gIndex.pack(0, 0))).to.be.false;
    expect(await gIndex.isRoot(await gIndex.pack(42, 0))).to.be.false;
    expect(await gIndex.isRoot(await gIndex.pack(42, 4))).to.be.false;
    expect(await gIndex.isRoot(await gIndex.pack(2048, 4))).to.be.false;

    const maxUint248 = BigInt(2) ** BigInt(248) - BigInt(1);
    expect(await gIndex.isRoot(await gIndex.pack(maxUint248, 255))).to.be.false;
  });

  it("test_concat", async () => {
    expect(await gIndex.unwrap(await gIndex.concat(await gIndex.pack(2, 99), await gIndex.pack(3, 99)))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5, 99)),
    );
    expect(await gIndex.unwrap(await gIndex.concat(await gIndex.pack(31, 99), await gIndex.pack(3, 99)))).to.equal(
      await gIndex.unwrap(await gIndex.pack(63, 99)),
    );
    expect(await gIndex.unwrap(await gIndex.concat(await gIndex.pack(31, 99), await gIndex.pack(6, 99)))).to.equal(
      await gIndex.unwrap(await gIndex.pack(126, 99)),
    );

    expect(
      await gIndex.unwrap(
        await gIndex.concat(
          await gIndex.concat(await gIndex.concat(ROOT, await gIndex.pack(2, 1)), await gIndex.pack(5, 1)),
          await gIndex.pack(9, 1),
        ),
      ),
    ).to.equal(await gIndex.unwrap(await gIndex.pack(73, 1)));

    expect(
      await gIndex.unwrap(
        await gIndex.concat(
          await gIndex.concat(await gIndex.concat(ROOT, await gIndex.pack(2, 9)), await gIndex.pack(5, 1)),
          await gIndex.pack(9, 4),
        ),
      ),
    ).to.equal(await gIndex.unwrap(await gIndex.pack(73, 4)));

    expect(await gIndex.unwrap(await gIndex.concat(ROOT, MAX))).to.equal(await gIndex.unwrap(MAX));
  });

  it("test_concat_RevertsIfZeroGIndex", async () => {
    await expect(gIndex.concat(ZERO, await gIndex.pack(1024, 1))).to.be.revertedWithCustomError(
      gIndex,
      "IndexOutOfRange",
    );
    await expect(gIndex.concat(await gIndex.pack(1024, 1), ZERO)).to.be.revertedWithCustomError(
      gIndex,
      "IndexOutOfRange",
    );
  });

  it("test_concat_BigIndicesBorderCases", async () => {
    await gIndex.concat(await gIndex.pack(2n ** 9n, 0), await gIndex.pack(2n ** 238n, 0));
    await gIndex.concat(await gIndex.pack(2n ** 47n, 0), await gIndex.pack(2n ** 200n, 0));
    await gIndex.concat(await gIndex.pack(2n ** 199n, 0), await gIndex.pack(2n ** 48n, 0));
  });

  it("test_concat_RevertsIfTooBigIndices", async () => {
    await expect(gIndex.concat(MAX, MAX)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");

    await expect(
      gIndex.concat(await gIndex.pack(2n ** 48n, 0), await gIndex.pack(2n ** 200n, 0)),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");

    await expect(
      gIndex.concat(await gIndex.pack(2n ** 200n, 0), await gIndex.pack(2n ** 48n, 0)),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
  });

  it("testFuzz_concat_WithRoot", async () => {
    for (let i = 0; i < 10; i++) {
      const randomIndex = (BigInt(hexlify(randomBytes(30))) % BigInt(2) ** BigInt(240)) + BigInt(1);
      const randomGIndex = await gIndex.wrap(zeroPadValue(toBeHex(randomIndex), 32));

      expect(await gIndex.unwrap(await gIndex.concat(ROOT, randomGIndex))).to.equal(
        await gIndex.unwrap(randomGIndex),
        "`concat` with a root should return right-hand side value",
      );
    }
  });

  it("testFuzz_unpack", async () => {
    for (let i = 0; i < 20; i++) {
      const index = BigInt(ethers.hexlify(randomBytes(30))) % BigInt(2) ** BigInt(240);
      const pow = BigInt(ethers.hexlify(randomBytes(1))) % 256n;

      const packed = await gIndex.pack(index, pow);

      expect(await gIndex.index(packed)).to.equal(index);
      expect(await gIndex.width(packed)).to.equal(2n ** pow);
    }
  });

  it("test_shr", async () => {
    let gI = await gIndex.pack(1024, 4);
    expect(await gIndex.unwrap(await gIndex.shr(gI, 0))).to.equal(await gIndex.unwrap(await gIndex.pack(1024, 4)));
    expect(await gIndex.unwrap(await gIndex.shr(gI, 1))).to.equal(await gIndex.unwrap(await gIndex.pack(1025, 4)));
    expect(await gIndex.unwrap(await gIndex.shr(gI, 15))).to.equal(await gIndex.unwrap(await gIndex.pack(1039, 4)));

    gI = await gIndex.pack(1031, 4);
    expect(await gIndex.unwrap(await gIndex.shr(gI, 0))).to.equal(await gIndex.unwrap(await gIndex.pack(1031, 4)));
    expect(await gIndex.unwrap(await gIndex.shr(gI, 1))).to.equal(await gIndex.unwrap(await gIndex.pack(1032, 4)));
    expect(await gIndex.unwrap(await gIndex.shr(gI, 8))).to.equal(await gIndex.unwrap(await gIndex.pack(1039, 4)));

    gI = await gIndex.pack(2049, 4);
    expect(await gIndex.unwrap(await gIndex.shr(gI, 0))).to.equal(await gIndex.unwrap(await gIndex.pack(2049, 4)));
    expect(await gIndex.unwrap(await gIndex.shr(gI, 1))).to.equal(await gIndex.unwrap(await gIndex.pack(2050, 4)));
    expect(await gIndex.unwrap(await gIndex.shr(gI, 14))).to.equal(await gIndex.unwrap(await gIndex.pack(2063, 4)));
  });

  it("test_shr_AfterConcat", async () => {
    const gIParent = await gIndex.pack(5, 4);

    let gI = await gIndex.pack(1024, 4);
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 0))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5120, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 1))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5121, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 15))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5135, 4)),
    );

    gI = await gIndex.pack(1031, 4);
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 0))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5127, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 1))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5128, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 8))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5135, 4)),
    );

    gI = await gIndex.pack(2049, 4);
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 0))).to.equal(
      await gIndex.unwrap(await gIndex.pack(10241, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 1))).to.equal(
      await gIndex.unwrap(await gIndex.pack(10242, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shr(await gIndex.concat(gIParent, gI), 14))).to.equal(
      await gIndex.unwrap(await gIndex.pack(10255, 4)),
    );
  });

  it("test_shr_OffTheWidth", async () => {
    await expect(gIndex.shr(ROOT, 1)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(gIndex.shr(await gIndex.pack(1024, 4), 16)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(gIndex.shr(await gIndex.pack(1031, 4), 9)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(gIndex.shr(await gIndex.pack(1023, 4), 1)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
  });

  it("test_shr_OffTheWidth_AfterConcat", async () => {
    const gIParent = await gIndex.pack(154, 4);

    await expect(gIndex.shr(await gIndex.concat(gIParent, ROOT), 1)).to.be.revertedWithCustomError(
      gIndex,
      "IndexOutOfRange",
    );
    await expect(
      gIndex.shr(await gIndex.concat(gIParent, await gIndex.pack(1024, 4)), 16),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(
      gIndex.shr(await gIndex.concat(gIParent, await gIndex.pack(1031, 4)), 9),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(
      gIndex.shr(await gIndex.concat(gIParent, await gIndex.pack(1023, 4)), 1),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
  });

  it("test_shl", async () => {
    let gI = await gIndex.pack(1023, 4);
    expect(await gIndex.unwrap(await gIndex.shl(gI, 0))).to.equal(await gIndex.unwrap(await gIndex.pack(1023, 4)));
    expect(await gIndex.unwrap(await gIndex.shl(gI, 1))).to.equal(await gIndex.unwrap(await gIndex.pack(1022, 4)));
    expect(await gIndex.unwrap(await gIndex.shl(gI, 15))).to.equal(await gIndex.unwrap(await gIndex.pack(1008, 4)));

    gI = await gIndex.pack(1031, 4);
    expect(await gIndex.unwrap(await gIndex.shl(gI, 0))).to.equal(await gIndex.unwrap(await gIndex.pack(1031, 4)));
    expect(await gIndex.unwrap(await gIndex.shl(gI, 1))).to.equal(await gIndex.unwrap(await gIndex.pack(1030, 4)));
    expect(await gIndex.unwrap(await gIndex.shl(gI, 7))).to.equal(await gIndex.unwrap(await gIndex.pack(1024, 4)));

    gI = await gIndex.pack(2063, 4);
    expect(await gIndex.unwrap(await gIndex.shl(gI, 0))).to.equal(await gIndex.unwrap(await gIndex.pack(2063, 4)));
    expect(await gIndex.unwrap(await gIndex.shl(gI, 1))).to.equal(await gIndex.unwrap(await gIndex.pack(2062, 4)));
    expect(await gIndex.unwrap(await gIndex.shl(gI, 15))).to.equal(await gIndex.unwrap(await gIndex.pack(2048, 4)));
  });

  it("test_shl_AfterConcat", async () => {
    const gIParent = await gIndex.pack(5, 4);

    let gI = await gIndex.pack(1023, 4);
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 0))).to.equal(
      await gIndex.unwrap(await gIndex.pack(3071, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 1))).to.equal(
      await gIndex.unwrap(await gIndex.pack(3070, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 15))).to.equal(
      await gIndex.unwrap(await gIndex.pack(3056, 4)),
    );

    gI = await gIndex.pack(1031, 4);
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 0))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5127, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 1))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5126, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 7))).to.equal(
      await gIndex.unwrap(await gIndex.pack(5120, 4)),
    );

    gI = await gIndex.pack(2063, 4);
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 0))).to.equal(
      await gIndex.unwrap(await gIndex.pack(10255, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 1))).to.equal(
      await gIndex.unwrap(await gIndex.pack(10254, 4)),
    );
    expect(await gIndex.unwrap(await gIndex.shl(await gIndex.concat(gIParent, gI), 15))).to.equal(
      await gIndex.unwrap(await gIndex.pack(10240, 4)),
    );
  });

  it("test_shl_OffTheWidth", async () => {
    await expect(gIndex.shl(ROOT, 1)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(gIndex.shl(await gIndex.pack(1024, 4), 1)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(gIndex.shl(await gIndex.pack(1031, 4), 9)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(gIndex.shl(await gIndex.pack(1023, 4), 16)).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
  });

  it("test_shl_OffTheWidth_AfterConcat", async () => {
    const gIParent = await gIndex.pack(154, 4);

    await expect(gIndex.shl(await gIndex.concat(gIParent, ROOT), 1)).to.be.revertedWithCustomError(
      gIndex,
      "IndexOutOfRange",
    );
    await expect(
      gIndex.shl(await gIndex.concat(gIParent, await gIndex.pack(1024, 4)), 1),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(
      gIndex.shl(await gIndex.concat(gIParent, await gIndex.pack(1031, 4)), 9),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
    await expect(
      gIndex.shl(await gIndex.concat(gIParent, await gIndex.pack(1023, 4)), 16),
    ).to.be.revertedWithCustomError(gIndex, "IndexOutOfRange");
  });

  it("test_fls", async () => {
    for (let i = 1; i < 255; i++) {
      expect(await gIndex.fls((1n << BigInt(i)) - 1n)).to.equal(BigInt(i - 1));
      expect(await gIndex.fls(1n << BigInt(i))).to.equal(BigInt(i));
      expect(await gIndex.fls((1n << BigInt(i)) + 1n)).to.equal(BigInt(i));
    }

    expect(await gIndex.fls(3n)).to.equal(1n); // 0011
    expect(await gIndex.fls(7n)).to.equal(2n); // 0111
    expect(await gIndex.fls(10n)).to.equal(3n); // 1010
    expect(await gIndex.fls(300n)).to.equal(8n); // 0001 0010 1100
    expect(await gIndex.fls(0n)).to.equal(256n);
  });
});
