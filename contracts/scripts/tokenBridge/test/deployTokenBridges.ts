import { ethers, upgrades } from "hardhat";

import { TokenBridge } from "../../../typechain-types";
import { SupportedChainIds } from "../../../common/supportedNetworks";
import { deployBridgedTokenBeacon } from "./deployBridgedTokenBeacon";
import { pauseTypeRoles, unpauseTypeRoles } from "../../../test/hardhat/common/constants";
import { generateRoleAssignments } from "contracts/common/helpers";
import { TOKEN_BRIDGE_ROLES } from "contracts/common/constants";

export async function deployTokenBridge(messageServiceAddress: string, verbose = false) {
  const [owner] = await ethers.getSigners();
  const chainIds = [SupportedChainIds.SEPOLIA, SupportedChainIds.LINEA_TESTNET];

  const roleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, owner.address, []);

  // Deploy beacon for bridged tokens
  const tokenBeacons = await deployBridgedTokenBeacon(verbose);

  // Deploying TokenBridges
  const TokenBridgeFactory = await ethers.getContractFactory("TokenBridge");

  const l1TokenBridge = (await upgrades.deployProxy(TokenBridgeFactory, [
    {
      defaultAdmin: owner.address,
      messageService: messageServiceAddress,
      tokenBeacon: await tokenBeacons.l1TokenBeacon.getAddress(),
      sourceChainId: chainIds[0],
      targetChainId: chainIds[1],
      reservedTokens: [],
      roleAddresses: roleAddresses,
      pauseTypeRoles: pauseTypeRoles,
      unpauseTypeRoles: unpauseTypeRoles,
    },
  ])) as unknown as TokenBridge;
  await l1TokenBridge.waitForDeployment();
  if (verbose) {
    console.log("L1TokenBridge deployed, at address:", await l1TokenBridge.getAddress());
  }

  const l2TokenBridge = (await upgrades.deployProxy(TokenBridgeFactory, [
    {
      defaultAdmin: owner.address,
      messageService: messageServiceAddress,
      tokenBeacon: await tokenBeacons.l2TokenBeacon.getAddress(),
      sourceChainId: chainIds[1],
      targetChainId: chainIds[0],
      reservedTokens: [],
      roleAddresses: roleAddresses,
      pauseTypeRoles: pauseTypeRoles,
      unpauseTypeRoles: unpauseTypeRoles,
    },
  ])) as unknown as TokenBridge;
  await l2TokenBridge.waitForDeployment();
  if (verbose) {
    console.log("L2TokenBridge deployed, at address:", await l2TokenBridge.getAddress());
  }

  // Setting reciprocal addresses of TokenBridges
  await l1TokenBridge.setRemoteTokenBridge(await l2TokenBridge.getAddress());
  await l2TokenBridge.setRemoteTokenBridge(await l1TokenBridge.getAddress());
  if (verbose) {
    console.log("Reciprocal addresses of TokenBridges set");
  }

  if (verbose) {
    console.log("Deployment finished");
  }

  return { l1TokenBridge, l2TokenBridge, chainIds, ...tokenBeacons };
}

export async function deployTokenBridgeWithMockMessaging(verbose = false) {
  const MessageServiceFactory = await ethers.getContractFactory("MockMessageService");

  // Deploying mock messaging service
  const messageService = await MessageServiceFactory.deploy();
  await messageService.waitForDeployment();

  const deploymentVars = await deployTokenBridge(await messageService.getAddress(), verbose);
  return { messageService, ...deploymentVars };
}
