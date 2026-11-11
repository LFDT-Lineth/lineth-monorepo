import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";
import { type Address, getContractAddress } from "viem";

import { deployBridgedTokenBeacon } from "./deployBridgedTokenBeacon.js";
import { TOKEN_BRIDGE_ROLES } from "../../../common/constants/index.js";
import { generateRoleAssignments } from "../../../common/helpers/index.js";
import { SupportedChainIds } from "../../../common/supportedNetworks.js";
import { pauseTypeRoles, unpauseTypeRoles } from "../../../test/hardhat/common/constants/index.js";

import type { PauseTypeRole, RoleAddress } from "../../../test/hardhat/common/types.js";

export async function deployTokenBridge(messageServiceAddress: Address, verbose = false) {
  const connection = await network.getOrCreate();
  const { viem } = connection;
  const oz = await upgrades(hre, connection);
  const publicClient = await viem.getPublicClient();
  const [owner] = await viem.getWalletClients();
  const ownerAddress = owner.account.address;
  const chainIds = [SupportedChainIds.SEPOLIA, SupportedChainIds.LINEA_TESTNET];

  const roleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, ownerAddress, []);

  // Deploy beacon for bridged tokens
  const tokenBeacons = await deployBridgedTokenBeacon(verbose);

  // Deploying TokenBridges. The implementation is deployed up front so each deployProxy below is exactly one owner
  // transaction (OZ Upgrades v4 proxies create their ProxyAdmin inside the proxy constructor), which keeps the
  // counterfactual address of l2TokenBridge (owner nonce + 1) correct.
  await oz.deployImplementation("TokenBridge");

  const l1TokenBridgeInitializationData: TokenBridgeInitializationData = {
    defaultAdmin: ownerAddress,
    messageService: messageServiceAddress,
    tokenBeacon: tokenBeacons.l1TokenBeacon.address,
    sourceChainId: chainIds[0],
    targetChainId: chainIds[1],
    reservedTokens: [],
    remoteSender: getContractAddress({
      from: ownerAddress,
      nonce: 1n + BigInt(await publicClient.getTransactionCount({ address: ownerAddress, blockTag: "pending" })),
    }), // Counterfactual address of l2TokenBridge
    roleAddresses: roleAddresses,
    pauseTypeRoles,
    unpauseTypeRoles,
  };

  const l1TokenBridge = await oz.deployProxy("TokenBridge", [l1TokenBridgeInitializationData]);
  if (verbose) {
    console.log("L1TokenBridge deployed, at address:", l1TokenBridge.address);
  }

  const l2TokenBridgeInitializationData: TokenBridgeInitializationData = {
    defaultAdmin: ownerAddress,
    messageService: messageServiceAddress,
    tokenBeacon: tokenBeacons.l2TokenBeacon.address,
    sourceChainId: chainIds[1],
    targetChainId: chainIds[0],
    reservedTokens: [],
    remoteSender: l1TokenBridge.address,
    roleAddresses: roleAddresses,
    pauseTypeRoles,
    unpauseTypeRoles,
  };

  const l2TokenBridge = await oz.deployProxy("TokenBridge", [l2TokenBridgeInitializationData]);
  if (verbose) {
    console.log("L2TokenBridge deployed, at address:", l2TokenBridge.address);
  }

  if (verbose) {
    console.log("Deployment finished");
  }

  return {
    l1TokenBridge,
    l2TokenBridge,
    chainIds,
    ...tokenBeacons,
    l1TokenBridgeInitializationData,
    l2TokenBridgeInitializationData,
  };
}

export type TokenBridgeInitializationData = {
  defaultAdmin: Address;
  messageService: Address;
  tokenBeacon: Address;
  sourceChainId: number;
  targetChainId: number;
  reservedTokens: Address[];
  remoteSender: Address;
  roleAddresses: RoleAddress[];
  pauseTypeRoles: PauseTypeRole[];
  unpauseTypeRoles: PauseTypeRole[];
};

export async function deployTokenBridgeWithMockMessaging(verbose = false) {
  const { viem } = await network.getOrCreate();

  // Deploying mock messaging service
  const messageService = await viem.deployContract("MockMessageService");

  const deploymentVars = await deployTokenBridge(messageService.address, verbose);
  return { messageService, ...deploymentVars };
}
