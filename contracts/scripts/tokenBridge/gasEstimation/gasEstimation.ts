import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";
import { getAddress, getContractAddress, maxUint256 } from "viem";

import { TOKEN_BRIDGE_ROLES } from "../../../common/constants/index.js";
import { generateRoleAssignments } from "../../../common/helpers/roles.js";
import { SupportedChainIds } from "../../../common/supportedNetworks.js";
import { getPermitData } from "../../../test/hardhat/bridging/token/helpers/permitHelper.js";
import { pauseTypeRoles, unpauseTypeRoles } from "../../../test/hardhat/common/constants/index.js";
import { deployBridgedTokenBeacon } from "../test/deployBridgedTokenBeacon.js";
import { deployTokens } from "../test/deployTokens.js";

import type { TokenBridgeInitializationData } from "../test/deployTokenBridges.js";

const initialUserBalance = BigInt(10 ** 9);
const bridgeAmount = 70;
const DEPLOYED_STATUS = getAddress("0x0000000000000000000000000000000000000333");
const deadline = maxUint256;

/**
 * Simple script to test the gas cost of the method bridgeToken
 */
async function main() {
  const connection = await network.getOrCreate();
  const { viem } = connection;
  const oz = await upgrades(hre, connection);
  const publicClient = await viem.getPublicClient();
  const [user] = await viem.getWalletClients();
  const userAddress = user.account.address;
  const chainId = await publicClient.getChainId();

  // Deploy beacon for bridged tokens
  const tokenBeacons = await deployBridgedTokenBeacon();

  // Deploy the messageService
  const mockMessageServiceV2 = await viem.deployContract("MockMessageServiceV2", []);

  // Deploy tokenBridges. The TokenBridge initializer now takes the remote bridge (`remoteSender`) instead of a
  // later setRemoteTokenBridge call, so the implementation is deployed up front and the L2 bridge address is
  // precomputed (owner nonce + 1), as in scripts/tokenBridge/test/deployTokenBridges.ts.
  const roleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, userAddress, []);
  const chainIds = [SupportedChainIds.SEPOLIA, SupportedChainIds.LINEA_TESTNET];
  // MockTokenBridge only adds a setter and inherits TokenBridge's initializer.
  const mockTokenBridgeOptions = { unsafeAllow: ["missing-initializer" as const] };
  await oz.deployImplementation("MockTokenBridge", mockTokenBridgeOptions);

  const l1TokenBridgeInitializationData: TokenBridgeInitializationData = {
    defaultAdmin: userAddress,
    messageService: mockMessageServiceV2.address,
    tokenBeacon: tokenBeacons.l1TokenBeacon.address,
    sourceChainId: chainIds[0],
    targetChainId: chainIds[1],
    reservedTokens: [],
    remoteSender: getContractAddress({
      from: userAddress,
      nonce: 1n + BigInt(await publicClient.getTransactionCount({ address: userAddress, blockTag: "pending" })),
    }), // Counterfactual address of l2TokenBridge
    roleAddresses,
    pauseTypeRoles,
    unpauseTypeRoles,
  };
  const l1TokenBridge = await oz.deployProxy(
    "MockTokenBridge",
    [l1TokenBridgeInitializationData],
    mockTokenBridgeOptions,
  );

  await oz.deployProxy(
    "MockTokenBridge",
    [
      {
        ...l1TokenBridgeInitializationData,
        tokenBeacon: tokenBeacons.l2TokenBeacon.address,
        sourceChainId: chainIds[1],
        targetChainId: chainIds[0],
        remoteSender: l1TokenBridge.address,
      },
    ],
    mockTokenBridgeOptions,
  );

  // Deploy test tokens
  const tokens = await deployTokens(false);

  // Mint tokens for user and approve bridge
  for (const name in tokens) {
    const token = tokens[name];
    await publicClient.waitForTransactionReceipt({
      hash: await token.write.mint([userAddress, initialUserBalance]),
    });
    await publicClient.waitForTransactionReceipt({
      hash: await token.write.approve([l1TokenBridge.address, maxUint256], { account: user.account }),
    });
  }

  // Create a bridgedToken to test to bridge with the metadata
  const abcToken = await oz.deployBeaconProxy(tokenBeacons.l1TokenBeacon.address, "BridgedToken", [
    "AbcTokendfgrdgredt",
    "ABC",
    18,
  ]);

  // Estimate gas cost without permitData
  let gasCost = await l1TokenBridge.estimateGas.bridgeToken([tokens.L1DAI.address, 10n, userAddress], {
    account: user.account,
  });
  console.log("basic bridgeToken:                           ", gasCost.toString());

  // Prepare data for permit calldata
  await publicClient.waitForTransactionReceipt({
    hash: await abcToken.write.mint([userAddress, initialUserBalance]),
  });
  const nonce = await abcToken.read.nonces([userAddress]);
  const permitData = await getPermitData(user, abcToken, nonce, chainId, l1TokenBridge.address, bridgeAmount, deadline);

  gasCost = await l1TokenBridge.estimateGas.bridgeTokenWithPermit(
    [abcToken.address, BigInt(bridgeAmount), userAddress, permitData],
    { account: user.account },
  );
  console.log("bridgeToken with permit:                     ", gasCost.toString());

  await publicClient.waitForTransactionReceipt({
    hash: await l1TokenBridge.write.setNativeMappingValue([tokens.L1DAI.address, DEPLOYED_STATUS]),
  });

  // console.log(await l1TokenBridge.read.nativeToBridgedToken([tokens.L1DAI.address]));

  gasCost = await l1TokenBridge.estimateGas.bridgeToken([tokens.L1DAI.address, 10n, userAddress], {
    account: user.account,
  });
  console.log("bridgeToken after confirmDeploy:             ", gasCost.toString());

  await publicClient.waitForTransactionReceipt({
    hash: await l1TokenBridge.write.setNativeMappingValue([abcToken.address, DEPLOYED_STATUS]),
  });

  gasCost = await l1TokenBridge.estimateGas.bridgeTokenWithPermit(
    [abcToken.address, BigInt(bridgeAmount), userAddress, permitData],
    { account: user.account },
  );
  console.log("bridgeToken with permit after confirmDeploy: ", gasCost.toString());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
