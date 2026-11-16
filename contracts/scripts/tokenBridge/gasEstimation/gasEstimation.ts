import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import hre, { network } from "hardhat";
import { getAddress, maxUint256 } from "viem";

import { waitForSuccessfulReceipt } from "../../../common/helpers/receipt.js";
import { getPermitData } from "../../../test/hardhat/bridging/token/helpers/permitHelper.js";
import { deployTokenBridge } from "../test/deployTokenBridges.js";
import { deployTokens } from "../test/deployTokens.js";

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

  const mockMessageServiceV2 = await viem.deployContract("MockMessageServiceV2", []);
  const { l1TokenBridge, l1TokenBeacon } = await deployTokenBridge(
    mockMessageServiceV2.address,
    false,
    "MockTokenBridge",
  );

  // Deploy test tokens
  const tokens = await deployTokens(false);

  // Mint tokens for user and approve bridge
  for (const name in tokens) {
    const token = tokens[name];
    await waitForSuccessfulReceipt(publicClient, await token.write.mint([userAddress, initialUserBalance]));
    await waitForSuccessfulReceipt(
      publicClient,
      await token.write.approve([l1TokenBridge.address, maxUint256], { account: user.account }),
    );
  }

  // Create a bridgedToken to test to bridge with the metadata
  const abcToken = await oz.deployBeaconProxy(l1TokenBeacon.address, "BridgedToken", ["AbcTokendfgrdgredt", "ABC", 18]);

  // Estimate gas cost without permitData
  let gasCost = await l1TokenBridge.estimateGas.bridgeToken([tokens.L1DAI.address, 10n, userAddress], {
    account: user.account,
  });
  console.log("basic bridgeToken:                           ", gasCost.toString());

  // Prepare data for permit calldata
  await waitForSuccessfulReceipt(publicClient, await abcToken.write.mint([userAddress, initialUserBalance]));
  const nonce = await abcToken.read.nonces([userAddress]);
  const permitData = await getPermitData(user, abcToken, nonce, chainId, l1TokenBridge.address, bridgeAmount, deadline);

  gasCost = await l1TokenBridge.estimateGas.bridgeTokenWithPermit(
    [abcToken.address, BigInt(bridgeAmount), userAddress, permitData],
    { account: user.account },
  );
  console.log("bridgeToken with permit:                     ", gasCost.toString());

  await waitForSuccessfulReceipt(
    publicClient,
    await l1TokenBridge.write.setNativeMappingValue([tokens.L1DAI.address, DEPLOYED_STATUS]),
  );

  // console.log(await l1TokenBridge.read.nativeToBridgedToken([tokens.L1DAI.address]));

  gasCost = await l1TokenBridge.estimateGas.bridgeToken([tokens.L1DAI.address, 10n, userAddress], {
    account: user.account,
  });
  console.log("bridgeToken after confirmDeploy:             ", gasCost.toString());

  await waitForSuccessfulReceipt(
    publicClient,
    await l1TokenBridge.write.setNativeMappingValue([abcToken.address, DEPLOYED_STATUS]),
  );

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
