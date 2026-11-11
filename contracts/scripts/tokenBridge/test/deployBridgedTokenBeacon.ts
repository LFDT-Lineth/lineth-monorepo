import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";
import { getAddress, getContract } from "viem";

import { pinnedUpgradeableBeaconArtifact } from "../../../ignition/modules/lib/pinned.js";

/**
 * Deploys one BridgedToken implementation behind two pinned OpenZeppelin 4.x `UpgradeableBeacon`s (L1 and L2): the
 * exact beacon bytecode the Hardhat 2 OZ plugin deployed in production. OZ Upgrades still validates the implementation,
 * but its `deployBeacon` is not used: since plugin v3 it deploys an OpenZeppelin 5 beacon, whose owner checks revert
 * differently.
 */
export async function deployBridgedTokenBeacon(verbose = false) {
  const connection = await network.getOrCreate();
  const { viem } = connection;
  const oz = await upgrades(hre, connection);
  const publicClient = await viem.getPublicClient();
  const [deployer] = await viem.getWalletClients();

  await oz.validateImplementation("BridgedToken", { kind: "beacon" });
  const implementation = await viem.deployContract("BridgedToken");

  const deployBeacon = async () => {
    const hash = await deployer.deployContract({
      abi: pinnedUpgradeableBeaconArtifact.abi,
      bytecode: pinnedUpgradeableBeaconArtifact.bytecode,
      args: [implementation.address],
    });
    const { contractAddress } = await publicClient.waitForTransactionReceipt({ hash });
    if (!contractAddress) {
      throw new Error("UpgradeableBeacon deployment produced no contract address");
    }
    return getContract({
      address: getAddress(contractAddress),
      abi: pinnedUpgradeableBeaconArtifact.abi,
      client: { public: publicClient, wallet: deployer },
    });
  };

  const l1TokenBeacon = await deployBeacon();
  if (verbose) {
    console.log("L1TokenBeacon deployed, at address:", l1TokenBeacon.address);
  }

  const l2TokenBeacon = await deployBeacon();
  if (verbose) {
    console.log("L2TokenBeacon deployed, at address:", l2TokenBeacon.address);
  }

  return { l1TokenBeacon, l2TokenBeacon };
}
