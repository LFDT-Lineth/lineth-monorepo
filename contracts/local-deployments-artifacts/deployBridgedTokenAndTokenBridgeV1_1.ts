import {
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "./static-artifacts/ProxyAdmin.json";
import {
  contractName as BridgedTokenContractName,
  abi as BridgedTokenAbi,
  bytecode as BridgedTokenBytecode,
} from "./dynamic-artifacts/BridgedToken.json";
import {
  contractName as TokenBridgeContractName,
  abi as TokenBridgeAbi,
  bytecode as TokenBridgeBytecode,
} from "./dynamic-artifacts/TokenBridgeV1_1.json";
import {
  contractName as UpgradeableBeaconContractName,
  abi as UpgradeableBeaconAbi,
  bytecode as UpgradeableBeaconBytecode,
} from "./static-artifacts/UpgradeableBeacon.json";

import {
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "./static-artifacts/TransparentUpgradeableProxy.json";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment";
import { generateRoleAssignments } from "../common/helpers/roles";
import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "contracts/common/constants";
import { ethers } from "ethers";
import { deployContractFromArtifacts, getInitializerData } from "../common/helpers/deployments";

async function main() {
  const ORDERED_NONCE_POST_L2MESSAGESERVICE = 3;
  const ORDERED_NONCE_POST_LINEAROLLUP = 4;

  let securityCouncilAddress;

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    securityCouncilAddress = getRequiredEnvVar("L1_TOKEN_BRIDGE_SECURITY_COUNCIL");
  } else {
    securityCouncilAddress = getRequiredEnvVar("L2_TOKEN_BRIDGE_SECURITY_COUNCIL");
  }

  const l2MessageServiceAddress = process.env.L2MESSAGESERVICE_ADDRESS;
  const lineaRollupAddress = process.env.LINEA_ROLLUP_ADDRESS;

  const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");

  const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, securityCouncilAddress, []);
  const roleAddresses = getEnvVarOrDefault("TOKEN_BRIDGE_ROLE_ADDRESSES", defaultRoleAddresses);
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.PRIVATE_KEY!, provider);

  let walletNonce;
  let remoteDeployerNonce;
  let fees = {};

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    walletNonce = await getL1DeployerNonce();
    remoteDeployerNonce = await getL2DeployerNonce();
  } else {
    walletNonce = await getL2DeployerNonce();
    remoteDeployerNonce = await getL1DeployerNonce();
    fees = {
      maxFeePerGas: 7_200_000_000_000n,
      maxPriorityFeePerGas: 7_000_000_000_000n,
    };
  }

  async function getL1DeployerNonce(): Promise<number> {
    if (!process.env.L1_NONCE) {
      return await wallet.getNonce();
    } else {
      return parseInt(process.env.L1_NONCE) + ORDERED_NONCE_POST_LINEAROLLUP;
    }
  }

  async function getL2DeployerNonce(): Promise<number> {
    if (!process.env.L2_NONCE) {
      return await wallet.getNonce();
    } else {
      return parseInt(process.env.L2_NONCE) + ORDERED_NONCE_POST_L2MESSAGESERVICE;
    }
  }

  const tokenBridgeContractImplementationName = "tokenBridgeContractImplementation";

  const [bridgedToken, tokenBridgeImplementation, proxyAdmin] = await Promise.all([
    deployContractFromArtifacts(BridgedTokenContractName, BridgedTokenAbi, BridgedTokenBytecode, wallet, {
      nonce: walletNonce,
      ...fees,
    }),
    deployContractFromArtifacts(tokenBridgeContractImplementationName, TokenBridgeAbi, TokenBridgeBytecode, wallet, {
      nonce: walletNonce + 1,
      ...fees,
    }),
    deployContractFromArtifacts(ProxyAdminContractName, ProxyAdminAbi, ProxyAdminBytecode, wallet, {
      nonce: walletNonce + 2,
      ...fees,
    }),
  ]);

  const bridgedTokenAddress = await bridgedToken.getAddress();
  const tokenBridgeImplementationAddress = await tokenBridgeImplementation.getAddress();
  const proxyAdminAddress = await proxyAdmin.getAddress();

  const chainId = (await provider.getNetwork()).chainId;

  console.log(`Deploying UpgradeableBeacon: chainId=${chainId} bridgedTokenAddress=${bridgedTokenAddress}`);

  const beaconProxy = await deployContractFromArtifacts(
    UpgradeableBeaconContractName,
    UpgradeableBeaconAbi,
    UpgradeableBeaconBytecode,
    wallet,
    bridgedTokenAddress,
    fees,
  );

  const beaconProxyAddress = await beaconProxy.getAddress();

  let deployingChainMessageService = l2MessageServiceAddress;
  let reservedAddresses = process.env.L2_RESERVED_TOKEN_ADDRESSES
    ? process.env.L2_RESERVED_TOKEN_ADDRESSES.split(",")
    : [];
  const remoteSender = ethers.getCreateAddress({
    from: process.env.REMOTE_DEPLOYER_ADDRESS || "",
    nonce: remoteDeployerNonce + 4,
  });

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    console.log(
      `TOKEN_BRIDGE_L1=${process.env.TOKEN_BRIDGE_L1}. Deploying TokenBridge on L1, using L1_RESERVED_TOKEN_ADDRESSES environment variable and remoteSender=${remoteSender}`,
    );
    deployingChainMessageService = lineaRollupAddress;
    reservedAddresses = process.env.L1_RESERVED_TOKEN_ADDRESSES
      ? process.env.L1_RESERVED_TOKEN_ADDRESSES.split(",")
      : [];
  } else {
    console.log(
      `TOKEN_BRIDGE_L1=${process.env.TOKEN_BRIDGE_L1}. Deploying TokenBridge on L2, using L2_RESERVED_TOKEN_ADDRESSES environment variable and remoteSender=${remoteSender}`,
    );
  }

  const initializer = getInitializerData(TokenBridgeAbi, "initialize", [
    {
      defaultAdmin: securityCouncilAddress,
      messageService: deployingChainMessageService,
      tokenBeacon: beaconProxyAddress,
      sourceChainId: chainId,
      targetChainId: remoteChainId,
      remoteSender: remoteSender,
      reservedTokens: reservedAddresses,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
    },
  ]);

  await deployContractFromArtifacts(
    TokenBridgeContractName,
    TransparentUpgradeableProxyAbi,
    TransparentUpgradeableProxyBytecode,
    wallet,
    tokenBridgeImplementationAddress,
    proxyAdminAddress,
    initializer,
    fees,
  );
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
