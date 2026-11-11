import { expect } from "chai";
import { network } from "hardhat";
import { describe, it } from "node:test";
import { getAddress } from "viem";

import { deployBridgedTokenBeacon } from "../../../../scripts/tokenBridge/test/deployBridgedTokenBeacon.js";
import { getUpgrades } from "../../common/deployment.js";
import { getTestAccounts, expectRevertWithCustomError, expectRevertWithReason } from "../../common/helpers/index.js";

const initialUserBalance = 10000n;

async function createTokenBeaconProxy() {
  const { viem } = await network.getOrCreate();
  const oz = await getUpgrades();
  const [admin, unknown] = await getTestAccounts();

  // Deploy token beacons (pinned OpenZeppelin 4.x UpgradeableBeacon, as in production)
  const { l1TokenBeacon, l2TokenBeacon } = await deployBridgedTokenBeacon();

  // Create tokens
  const abcToken = await oz.deployBeaconProxy(l1TokenBeacon.address, "BridgedToken", ["AbcToken", "ABC", 18]);

  const sixDecimalsToken = await oz.deployBeaconProxy(l1TokenBeacon.address, "BridgedToken", [
    "sixDecimalsToken",
    "SIX",
    6,
  ]);

  // Create a new token implementation
  const newImplementation = await viem.deployContract("UpgradedBridgedToken");

  // Update l2TokenBeacon with new implementation
  await l2TokenBeacon.write.upgradeTo([newImplementation.address], { account: admin.account });

  // Set initial balance
  await sixDecimalsToken.write.mint([unknown.address, initialUserBalance], { account: admin.account });

  return {
    admin,
    unknown,
    l1TokenBeacon,
    l2TokenBeacon,
    newImplementation,
    abcToken,
    sixDecimalsToken,
  };
}

async function loadTokenBeaconProxyFixture() {
  const { networkHelpers } = await network.getOrCreate();
  return networkHelpers.loadFixture(createTokenBeaconProxy);
}

describe("BridgedToken", function () {
  it("Should deploy BridgedToken", async function () {
    const { abcToken, sixDecimalsToken } = await loadTokenBeaconProxyFixture();
    expect(abcToken.address).to.be.not.null;
    expect(sixDecimalsToken.address).to.be.not.null;
  });

  it("Should set the right metadata", async function () {
    const { abcToken, sixDecimalsToken } = await loadTokenBeaconProxyFixture();
    expect(await abcToken.read.name()).to.be.equal("AbcToken");
    expect(await abcToken.read.symbol()).to.be.equal("ABC");
    expect(await abcToken.read.decimals()).to.be.equal(18);
    expect(await sixDecimalsToken.read.name()).to.be.equal("sixDecimalsToken");
    expect(await sixDecimalsToken.read.symbol()).to.be.equal("SIX");
    expect(await sixDecimalsToken.read.decimals()).to.be.equal(6);
  });

  it("Should mint tokens", async function () {
    const { admin, unknown, abcToken } = await loadTokenBeaconProxyFixture();
    const amount = 100n;
    await abcToken.write.mint([unknown.address, amount], { account: admin.account });
    expect(await abcToken.read.balanceOf([unknown.address])).to.be.equal(amount);
  });

  it("Should burn tokens", async function () {
    const { admin, unknown, sixDecimalsToken } = await loadTokenBeaconProxyFixture();
    const amount = 100n;
    await sixDecimalsToken.write.approve([admin.address, amount], { account: unknown.account });
    await sixDecimalsToken.write.burn([unknown.address, amount], { account: admin.account });
    expect(await sixDecimalsToken.read.balanceOf([unknown.address])).to.be.equal(initialUserBalance - amount);
  });

  it("Should revert if mint/burn are called by an unknown address", async function () {
    const { unknown, abcToken } = await loadTokenBeaconProxyFixture();
    const amount = 100n;

    await expectRevertWithCustomError(
      abcToken,
      abcToken.write.mint([unknown.address, amount], { account: unknown.account }),
      "OnlyBridge",
    );
    await expectRevertWithCustomError(
      abcToken,
      abcToken.write.burn([unknown.address, amount], { account: unknown.account }),
      "OnlyBridge",
    );
  });
});

describe("BeaconProxy", function () {
  it("Should enable upgrade of existing beacon proxy", async function () {
    const { viem } = await network.getOrCreate();
    const { admin, l1TokenBeacon, abcToken, newImplementation } = await loadTokenBeaconProxyFixture();
    await l1TokenBeacon.write.upgradeTo([newImplementation.address], { account: admin.account });
    expect(await l1TokenBeacon.read.implementation()).to.be.equal(getAddress(newImplementation.address));
    expect(await (await viem.getContractAt("UpgradedBridgedToken", abcToken.address)).read.isUpgraded()).to.be.equal(
      true,
    );
  });

  it("Should deploy new beacon proxy with the updated implementation", async function () {
    const { l2TokenBeacon } = await loadTokenBeaconProxyFixture();
    const oz = await getUpgrades();
    const newTokenBeaconProxy = await oz.deployBeaconProxy(l2TokenBeacon.address, "UpgradedBridgedToken", [
      "NAME",
      "SYMBOL",
      18, // Decimals
    ]);
    expect(await newTokenBeaconProxy.read.isUpgraded()).to.be.equal(true);
  });

  it("Beacon upgrade should only be done by the owner", async function () {
    const { unknown, l1TokenBeacon, newImplementation } = await loadTokenBeaconProxyFixture();
    await expectRevertWithReason(
      l1TokenBeacon.write.upgradeTo([newImplementation.address], { account: unknown.account }),
      "Ownable: caller is not the owner",
    );
  });
});
