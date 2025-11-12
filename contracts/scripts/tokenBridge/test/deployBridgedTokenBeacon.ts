import { ethers, upgrades } from "hardhat";

export async function deployBridgedTokenBeacon(verbose = false) {
  const BridgedToken = await ethers.getContractFactory("BridgedToken");

  const l1TokenBeacon = await upgrades.deployBeacon(BridgedToken);
  await l1TokenBeacon.waitForDeployment();

  if (verbose) {
    console.log("L1TokenBeacon deployed, at address:", await l1TokenBeacon.getAddress());
  }

  const l2TokenBeacon = await upgrades.deployBeacon(BridgedToken);
  await l2TokenBeacon.waitForDeployment();
  if (verbose) {
    console.log("L2TokenBeacon deployed, at address:", await l2TokenBeacon.getAddress());
  }

  return { l1TokenBeacon, l2TokenBeacon };
}
