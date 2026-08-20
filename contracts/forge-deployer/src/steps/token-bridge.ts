// Forge-safe fork of the local TokenBridge deployment. Deployments are
// serialized before advancing the planned nonce, and the remote TokenBridge
// address comes from the runner's deterministic plan.
import { ethers } from "ethers";

import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "../../../common/constants";
import {
  deployContractFromArtifacts,
  getDeployNonceFromEnv,
  getInitializerData,
} from "../../../common/helpers/deployments";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../../../common/helpers/environment";
import {
  FeeOverrides,
  resolveL2DeployFeeOverrides,
  resolveOneModelFeeOverrides,
} from "../../../common/helpers/feeOverrides";
import { generateRoleAssignments } from "../../../common/helpers/roles";
import {
  contractName as BridgedTokenContractName,
  abi as BridgedTokenAbi,
  bytecode as BridgedTokenBytecode,
} from "../../../local-deployments-artifacts/dynamic-artifacts/BridgedToken.json";
import {
  contractName as TokenBridgeContractName,
  abi as TokenBridgeAbi,
  bytecode as TokenBridgeBytecode,
} from "../../../local-deployments-artifacts/dynamic-artifacts/TokenBridgeV1_1.json";
import {
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "../../../local-deployments-artifacts/static-artifacts/ProxyAdmin.json";
import {
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "../../../local-deployments-artifacts/static-artifacts/TransparentUpgradeableProxy.json";
import {
  contractName as UpgradeableBeaconContractName,
  abi as UpgradeableBeaconAbi,
  bytecode as UpgradeableBeaconBytecode,
} from "../../../local-deployments-artifacts/static-artifacts/UpgradeableBeacon.json";

async function main(): Promise<void> {
  const deployOnL1 = process.env.TOKEN_BRIDGE_L1 === "true";
  const securityCouncilAddress = getRequiredEnvVar(deployOnL1 ? "L1_SECURITY_COUNCIL" : "L2_SECURITY_COUNCIL");
  const l2MessageServiceAddress = getRequiredEnvVar("L2_MESSAGE_SERVICE_ADDRESS");
  const linethRollupAddress = getRequiredEnvVar("LINETH_ROLLUP_ADDRESS");
  const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");
  const remoteSender = getRequiredEnvVar("REMOTE_TOKEN_BRIDGE_ADDRESS");
  const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
  const roleAddresses = getEnvVarOrDefault(
    "TOKEN_BRIDGE_ROLE_ADDRESSES",
    generateRoleAssignments(TOKEN_BRIDGE_ROLES, securityCouncilAddress, []),
  );

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(getRequiredEnvVar("DEPLOYER_PRIVATE_KEY"), provider);
  const startingNonce = await getDeployNonceFromEnv(wallet, deployOnL1 ? "L1_NONCE" : "L2_NONCE");
  const fees: FeeOverrides = deployOnL1
    ? await resolveOneModelFeeOverrides(provider, "L1_DEPLOY_GAS_PRICE_WEI")
    : resolveL2DeployFeeOverrides();

  const bridgedToken = await deployContractFromArtifacts(
    BridgedTokenContractName,
    BridgedTokenAbi,
    BridgedTokenBytecode,
    wallet,
    { nonce: startingNonce, ...fees },
  );
  const tokenBridgeImplementation = await deployContractFromArtifacts(
    "tokenBridgeContractImplementation",
    TokenBridgeAbi,
    TokenBridgeBytecode,
    wallet,
    { nonce: startingNonce + 1, ...fees },
  );
  const proxyAdmin = await deployContractFromArtifacts(
    ProxyAdminContractName,
    ProxyAdminAbi,
    ProxyAdminBytecode,
    wallet,
    { nonce: startingNonce + 2, ...fees },
  );
  const [bridgedTokenAddress, tokenBridgeImplementationAddress, proxyAdminAddress] = await Promise.all([
    bridgedToken.getAddress(),
    tokenBridgeImplementation.getAddress(),
    proxyAdmin.getAddress(),
  ]);

  const beacon = await deployContractFromArtifacts(
    UpgradeableBeaconContractName,
    UpgradeableBeaconAbi,
    UpgradeableBeaconBytecode,
    wallet,
    bridgedTokenAddress,
    { nonce: startingNonce + 3, ...fees },
  );
  const beaconAddress = await beacon.getAddress();
  const reservedAddressesVariable = deployOnL1 ? "L1_RESERVED_TOKEN_ADDRESSES" : "L2_RESERVED_TOKEN_ADDRESSES";
  const reservedAddresses = process.env[reservedAddressesVariable]
    ? process.env[reservedAddressesVariable]!.split(",")
    : [];
  const chainId = (await provider.getNetwork()).chainId;
  const initializer = getInitializerData(TokenBridgeAbi, "initialize", [
    {
      defaultAdmin: securityCouncilAddress,
      messageService: deployOnL1 ? linethRollupAddress : l2MessageServiceAddress,
      tokenBeacon: beaconAddress,
      sourceChainId: chainId,
      targetChainId: remoteChainId,
      remoteSender,
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
    { nonce: startingNonce + 4, ...fees },
  );
}

main().catch((error: unknown) => {
  console.error(error);
  process.exitCode = 1;
});
