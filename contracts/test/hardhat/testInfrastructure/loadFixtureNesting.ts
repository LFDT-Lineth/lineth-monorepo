import { expect } from "chai";
import { network } from "hardhat";
import { describe, it } from "node:test";

/**
 * Guards `patches/@nomicfoundation__hardhat-network-helpers@3.0.12.patch`. Unpatched, `loadFixture` writes a stale
 * snapshot list back after a fixture that nests another (cached) `loadFixture`, so a later `loadFixture` of a third
 * fixture whose snapshot that nested restore invalidated fails with HHE60013 ("error reverting the snapshot").
 * Many shared fixtures (yield, operational) compose by nesting `loadFixture`, as they did under Hardhat 2.
 */
async function innerFixture() {
  const { viem } = await network.getOrCreate();
  return viem.deployContract("TestERC20", ["Inner", "INR", 1n]);
}

async function otherFixture() {
  const { viem } = await network.getOrCreate();
  return viem.deployContract("TestERC20", ["Other", "OTH", 1n]);
}

async function outerFixture() {
  const { viem, networkHelpers } = await network.getOrCreate();
  const inner = await networkHelpers.loadFixture(innerFixture);
  const outer = await viem.deployContract("TestERC20", ["Outer", "OUT", 1n]);
  return { inner, outer };
}

describe("loadFixture nesting (hardhat-network-helpers patch)", () => {
  it("keeps the snapshot list consistent when a fixture nests a cached loadFixture", async () => {
    const { networkHelpers } = await network.getOrCreate();

    const inner = await networkHelpers.loadFixture(innerFixture);
    const other = await networkHelpers.loadFixture(otherFixture);
    // The nested loadFixture(innerFixture) restores the inner snapshot, which invalidates otherFixture's snapshot.
    const { inner: innerAgain } = await networkHelpers.loadFixture(outerFixture);
    expect(innerAgain.address).to.equal(inner.address);

    // Unpatched: throws HHE60013 because the stale otherFixture snapshot was written back.
    const otherAgain = await networkHelpers.loadFixture(otherFixture);
    expect(otherAgain.address).to.not.equal(undefined);
    expect(await otherAgain.read.name()).to.equal("Other");
    expect(other.address).to.not.equal(undefined);
  });
});
