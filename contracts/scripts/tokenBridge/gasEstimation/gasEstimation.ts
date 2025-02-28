import { ethers, upgrades } from "hardhat";
import { getPermitData } from "../../../test/hardhat/bridging/token/helpers/permitHelper";
import { BridgedToken, MockTokenBridge } from "../../../typechain-types";
import { deployBridgedTokenBeacon } from "../test/deployBridgedTokenBeacon";
import { deployTokens } from "../test/deployTokens";

const initialUserBalance = BigInt(10 ** 9);
const bridgeAmount = 70;
const DEPLOYED_STATUS = ethers.getAddress("0x0000000000000000000000000000000000000333");
const deadline = ethers.MaxUint256;

/**
 * Simple script to test the gas cost of the method bridgeToken
 */
async function main() {
  const [user] = await ethers.getSigners();
  const { chainId } = await ethers.provider.getNetwork();

  // Deploy beacon for bridged tokens
  const tokenBeacons = await deployBridgedTokenBeacon();

  // Deploy the messageService
  const MockMessageServiceV2 = await ethers.getContractFactory("MockMessageServiceV2");
  const mockMessageServiceV2 = await MockMessageServiceV2.deploy();
  await mockMessageServiceV2.waitForDeployment();

  // Deploy tokenBridges
  const TokenBridgeFactory = await ethers.getContractFactory("MockTokenBridge");
  const l1TokenBridge = (await upgrades.deployProxy(TokenBridgeFactory, [
    user.address,
    await mockMessageServiceV2.getAddress(),
    await tokenBeacons.l1TokenBeacon.getAddress(),
    [],
  ])) as unknown as MockTokenBridge;
  await l1TokenBridge.waitForDeployment();

  const l2TokenBridge = (await upgrades.deployProxy(TokenBridgeFactory, [
    user.address,
    await mockMessageServiceV2.getAddress(),
    await tokenBeacons.l2TokenBeacon.getAddress(),
    [],
  ])) as unknown as MockTokenBridge;
  await l2TokenBridge.waitForDeployment();

  // Setting reciprocal addresses of TokenBridges
  await l1TokenBridge.setRemoteTokenBridge(await l2TokenBridge.getAddress());
  await l2TokenBridge.setRemoteTokenBridge(await l1TokenBridge.getAddress());

  // Deploy test tokens
  const tokens = await deployTokens(false);

  // Mint tokens for user and approve bridge
  for (const name in tokens) {
    const token = tokens[name];
    await token.mint(user.address, initialUserBalance);
    await token.connect(user).approve(await l1TokenBridge.getAddress(), ethers.MaxUint256);
  }

  // Create a bridgedToken to test to bridge with the metadata
  const BridgedToken = await ethers.getContractFactory("BridgedToken");
  const abcToken = await upgrades.deployBeaconProxy(await tokenBeacons.l1TokenBeacon.getAddress(), BridgedToken, [
    "AbcTokendfgrdgredt",
    "ABC",
    18,
  ]);

  // Estimate gas cost without permitData
  let gasCost = await l1TokenBridge
    .connect(user)
    .bridgeToken.estimateGas(await tokens.L1DAI.getAddress(), 10, user.address);
  console.log("basic bridgeToken:                           ", gasCost.toString());

  // Prepare data for permit calldata
  await abcToken.mint(user.address, initialUserBalance);
  const nonce = await abcToken.nonces(user.address);
  const permitData = await getPermitData(
    user,
    abcToken as unknown as BridgedToken,
    nonce,
    parseInt(chainId.toString()),
    await l1TokenBridge.getAddress(),
    bridgeAmount,
    deadline,
  );

  gasCost = await l1TokenBridge
    .connect(user)
    .bridgeTokenWithPermit.estimateGas(await abcToken.getAddress(), bridgeAmount, user.address, permitData);
  console.log("bridgeToken with permit:                     ", gasCost.toString());

  await l1TokenBridge.setNativeMappingValue(await tokens.L1DAI.getAddress(), DEPLOYED_STATUS);

  // console.log(await l1TokenBridge.nativeToBridgedToken(tokens.L1DAI.address));

  gasCost = await l1TokenBridge
    .connect(user)
    .bridgeToken.estimateGas(await tokens.L1DAI.getAddress(), 10, user.address);
  console.log("bridgeToken after confirmDeploy:             ", gasCost.toString());

  await l1TokenBridge.setNativeMappingValue(await abcToken.getAddress(), DEPLOYED_STATUS);

  gasCost = await l1TokenBridge
    .connect(user)
    .bridgeTokenWithPermit.estimateGas(await abcToken.getAddress(), bridgeAmount, user.address, permitData);
  console.log("bridgeToken with permit after confirmDeploy: ", gasCost.toString());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
