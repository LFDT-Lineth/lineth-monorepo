import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers, upgrades } from "hardhat";
import { BridgedToken, UpgradedBridgedToken } from "../../../../typechain-types";

const initialUserBalance = 10000;

async function createTokenBeaconProxy() {
  const [admin, unknown] = await ethers.getSigners();

  const BridgedToken = await ethers.getContractFactory("BridgedToken");

  // Deploy token beacon
  const l1TokenBeacon = await upgrades.deployBeacon(BridgedToken);
  await l1TokenBeacon.waitForDeployment();

  const l2TokenBeacon = await upgrades.deployBeacon(BridgedToken);
  await l2TokenBeacon.waitForDeployment();

  // Create tokens
  const abcToken = (await upgrades.deployBeaconProxy(await l1TokenBeacon.getAddress(), BridgedToken, [
    "AbcToken",
    "ABC",
    18,
  ])) as unknown as BridgedToken;

  const sixDecimalsToken = (await upgrades.deployBeaconProxy(await l1TokenBeacon.getAddress(), BridgedToken, [
    "sixDecimalsToken",
    "SIX",
    6,
  ])) as unknown as BridgedToken;

  // Create a new token implementation
  const UpgradedBridgedToken = await ethers.getContractFactory("UpgradedBridgedToken");
  const newImplementation = await UpgradedBridgedToken.deploy();
  await newImplementation.waitForDeployment();

  // Update l2TokenBeacon with new implementation
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  await l2TokenBeacon.connect(admin).upgradeTo(newImplementation.getAddress());

  // Set initial balance
  await sixDecimalsToken.connect(admin).mint(unknown.address, initialUserBalance);

  return {
    admin,
    unknown,
    l1TokenBeacon,
    l2TokenBeacon,
    newImplementation,
    UpgradedBridgedToken,
    abcToken,
    sixDecimalsToken,
  };
}

describe("BridgedToken", function () {
  it("Should deploy BridgedToken", async function () {
    const { abcToken, sixDecimalsToken } = await loadFixture(createTokenBeaconProxy);
    expect(await abcToken.getAddress()).to.be.not.null;
    expect(await sixDecimalsToken.getAddress()).to.be.not.null;
  });

  it("Should set the right metadata", async function () {
    const { abcToken, sixDecimalsToken } = await loadFixture(createTokenBeaconProxy);
    expect(await abcToken.name()).to.be.equal("AbcToken");
    expect(await abcToken.symbol()).to.be.equal("ABC");
    expect(await abcToken.decimals()).to.be.equal(18);
    expect(await sixDecimalsToken.name()).to.be.equal("sixDecimalsToken");
    expect(await sixDecimalsToken.symbol()).to.be.equal("SIX");
    expect(await sixDecimalsToken.decimals()).to.be.equal(6);
  });

  it("Should mint tokens", async function () {
    const { admin, unknown, abcToken } = await loadFixture(createTokenBeaconProxy);
    const amount = 100;
    await abcToken.connect(admin).mint(unknown.address, amount);
    expect(await abcToken.balanceOf(unknown.address)).to.be.equal(amount);
  });

  it("Should burn tokens", async function () {
    const { admin, unknown, sixDecimalsToken } = await loadFixture(createTokenBeaconProxy);
    const amount = 100;
    await sixDecimalsToken.connect(unknown).approve(admin.address, amount);
    await sixDecimalsToken.connect(admin).burn(unknown.address, amount);
    expect(await sixDecimalsToken.balanceOf(unknown.address)).to.be.equal(initialUserBalance - amount);
  });

  it("Should revert if mint/burn are called by an unknown address", async function () {
    const { unknown, abcToken } = await loadFixture(createTokenBeaconProxy);
    const amount = 100;
    await expect(abcToken.connect(unknown).mint(unknown.address, amount)).to.be.revertedWithCustomError(
      abcToken,
      "OnlyBridge",
    );
    await expect(abcToken.connect(unknown).burn(unknown.address, amount)).to.be.revertedWithCustomError(
      abcToken,
      "OnlyBridge",
    );
  });
});

describe("BeaconProxy", function () {
  it("Should enable upgrade of existing beacon proxy", async function () {
    const { admin, l1TokenBeacon, abcToken, newImplementation, UpgradedBridgedToken } =
      await loadFixture(createTokenBeaconProxy);
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    await l1TokenBeacon.connect(admin).upgradeTo(await newImplementation.getAddress());
    expect(await l1TokenBeacon.implementation()).to.be.equal(await newImplementation.getAddress());
    expect(
      await (UpgradedBridgedToken.attach(await abcToken.getAddress()) as UpgradedBridgedToken).isUpgraded(),
    ).to.be.equal(true);
  });

  it("Should deploy new beacon proxy with the updated implementation", async function () {
    const { l2TokenBeacon, UpgradedBridgedToken } = await loadFixture(createTokenBeaconProxy);
    const newTokenBeaconProxy = await upgrades.deployBeaconProxy(
      await l2TokenBeacon.getAddress(),
      UpgradedBridgedToken,
      [
        "NAME",
        "SYMBOL",
        18, // Decimals
      ],
    );
    expect(await newTokenBeaconProxy.isUpgraded()).to.be.equal(true);
  });

  it("Beacon upgrade should only be done by the owner", async function () {
    const { unknown, l1TokenBeacon, newImplementation } = await loadFixture(createTokenBeaconProxy);
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    await expect(l1TokenBeacon.connect(unknown).upgradeTo(await newImplementation.getAddress())).to.be.revertedWith(
      "Ownable: caller is not the owner",
    );
  });
});
