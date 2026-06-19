// Forked from contracts/local-deployments-artifacts/deployBridgedTokenAndTokenBridgeV1_1.ts
// for the lineth-stack scaffold. Three changes vs upstream:
//
// 1. Every deploy is serialized via sequential `await` with no explicit nonce
//    overrides, so ethers manages nonces from live wallet state. This drops the
//    upstream `ORDERED_NONCE_POST_LINEAROLLUP` offset, which was stale for this
//    quickstart's deploy order and broke against any non-fresh deployer.
//
// 2. remoteSender is supplied directly via REMOTE_TOKEN_BRIDGE_ADDRESS env
//    var instead of derived from `remoteDeployerNonce + 4`. The upstream
//    derivation also depended on the stale offset and silently produced the
//    wrong cross-chain TokenBridge initialization.
//
// 3. Deploy fees go through the shared single-fee-model helper so the L1
//    TokenBridge follows the same L1_DEPLOY_GAS_PRICE_WEI policy as the V8
//    rollup deploy, and no transaction mixes legacy and EIP-1559 fields.
//
// Bind-mounted over the upstream path in the deploy-contracts compose service.
import {
  TOKEN_BRIDGE_PAUSE_TYPES_ROLES,
  TOKEN_BRIDGE_ROLES,
  TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES,
} from "contracts/common/constants";
import { ethers } from "ethers";

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
  contractName as ProxyAdminContractName,
  abi as ProxyAdminAbi,
  bytecode as ProxyAdminBytecode,
} from "./static-artifacts/ProxyAdmin.json";
import {
  abi as TransparentUpgradeableProxyAbi,
  bytecode as TransparentUpgradeableProxyBytecode,
} from "./static-artifacts/TransparentUpgradeableProxy.json";
import {
  contractName as UpgradeableBeaconContractName,
  abi as UpgradeableBeaconAbi,
  bytecode as UpgradeableBeaconBytecode,
} from "./static-artifacts/UpgradeableBeacon.json";
import { deployContractFromArtifacts, getInitializerData } from "../common/helpers/deployments";
import { getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment";
import { assertSingleFeeModel, FeeOverrides, resolveOneModelFeeOverrides } from "../common/helpers/feeOverrides";
import { generateRoleAssignments } from "../common/helpers/roles";

async function main() {
  let securityCouncilAddress;

  if (process.env.TOKEN_BRIDGE_L1 === "true") {
    securityCouncilAddress = getRequiredEnvVar("L1_SECURITY_COUNCIL");
  } else {
    securityCouncilAddress = getRequiredEnvVar("L2_SECURITY_COUNCIL");
  }

  const l2MessageServiceAddress = process.env.L2_MESSAGE_SERVICE_ADDRESS;
  const lineaRollupAddress = process.env.LINEA_ROLLUP_ADDRESS;

  const remoteChainId = getRequiredEnvVar("REMOTE_CHAIN_ID");
  // Forked: deploy-contracts.sh passes the remote TokenBridge proxy address
  // directly. For step 3 (L1 deploy) it's the precomputed L2 TokenBridge from
  // account setup; for step 4 (L2 deploy) it's the L1 TokenBridge just
  // deployed in step 3 (extracted from step 3's log).
  const remoteSender = getRequiredEnvVar("REMOTE_TOKEN_BRIDGE_ADDRESS");

  const pauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_PAUSE_TYPES_ROLES", TOKEN_BRIDGE_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES", TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES);
  const defaultRoleAddresses = generateRoleAssignments(TOKEN_BRIDGE_ROLES, securityCouncilAddress, []);
  const roleAddresses = getEnvVarOrDefault("TOKEN_BRIDGE_ROLE_ADDRESSES", defaultRoleAddresses);
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  // L1 follows the same single-fee-model policy as the V8 rollup deploy
  // (explicit L1_DEPLOY_GAS_PRICE_WEI, else provider fee data). L2 keeps the
  // local fixed EIP-1559 fee required for the local Besu boot, routed through
  // the same one-model-only validation so the two fields can never conflict.
  const fees: FeeOverrides =
    process.env.TOKEN_BRIDGE_L1 === "true"
      ? await resolveOneModelFeeOverrides(provider, "L1_DEPLOY_GAS_PRICE_WEI")
      : assertSingleFeeModel({
          maxFeePerGas: 7_200_000_000_000n,
          maxPriorityFeePerGas: 7_000_000_000_000n,
        });

  const tokenBridgeContractImplementationName = "tokenBridgeContractImplementation";

  // Forked: serialize the 3 deploys instead of Promise.all. Each await blocks
  // on tx receipt before the next is sent, letting ethers manage nonces from
  // the live wallet state. No explicit `nonce:` overrides needed.
  const bridgedToken = await deployContractFromArtifacts(
    BridgedTokenContractName,
    BridgedTokenAbi,
    BridgedTokenBytecode,
    wallet,
    fees,
  );
  const tokenBridgeImplementation = await deployContractFromArtifacts(
    tokenBridgeContractImplementationName,
    TokenBridgeAbi,
    TokenBridgeBytecode,
    wallet,
    fees,
  );
  const proxyAdmin = await deployContractFromArtifacts(
    ProxyAdminContractName,
    ProxyAdminAbi,
    ProxyAdminBytecode,
    wallet,
    fees,
  );

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
