import * as dotenv from "dotenv";
import { ethers } from "ethers";

import { abi as ValidiumV3StubAbi, bytecode as ValidiumV3StubBytecode } from "./dynamic-artifacts/ValidiumV3Stub.json";
import {
  contractName as AddressFilterContractName,
  abi as AddressFilterAbi,
  bytecode as AddressFilterBytecode,
} from "./static-artifacts/AddressFilter.json";
import {
  abi as ForcedTransactionGatewayAbi,
  bytecode as ForcedTransactionGatewayBytecode,
} from "./static-artifacts/ForcedTransactionGateway.json";
import {
  contractName as MimcAddressContractName,
  abi as MimcAddressAbi,
  bytecode as MimcAddressFilterBytecode,
} from "./static-artifacts/Mimc.json";
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
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  VALIDIUM_ROLES,
  OPERATOR_ROLE,
  YIELD_PROVIDER_STAKING_ROLE,
  ADDRESS_ZERO,
  DEAD_ADDRESS,
  PRECOMPILES_ADDRESSES,
  FORCED_TRANSACTION_SENDER_ROLE,
} from "../common/constants";
import { deployContractFromArtifacts, getDeployNonceFromEnv, getInitializerData } from "../common/helpers/deployments";
import { getBooleanEnvVarOrDefault, getEnvVarOrDefault, getRequiredEnvVar } from "../common/helpers/environment";
import { resolveOneModelFeeOverrides } from "../common/helpers/feeOverrides";
import {
  getDeploymentNetworkName,
  requireAddressesFromRegistryOrEnv,
  requireAddressFromRegistryOrEnv,
} from "../common/helpers/readAddress";
import { generateRoleAssignments } from "../common/helpers/roles";

dotenv.config();

async function main() {
  const networkName = getDeploymentNetworkName();
  const validiumInitialBlockHash = getRequiredEnvVar("INITIAL_L2_BLOCK_HASH");
  const validiumInitialL2BlockNumber = getRequiredEnvVar("INITIAL_L2_BLOCK_NUMBER");
  const validiumSecurityCouncil = requireAddressFromRegistryOrEnv(
    networkName,
    "L1_SECURITY_COUNCIL",
    "L1_SECURITY_COUNCIL",
  );
  const validiumOperators = requireAddressesFromRegistryOrEnv(networkName, "VALIDIUM_OPERATORS", "VALIDIUM_OPERATORS");
  const validiumRateLimitPeriodInSeconds = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_PERIOD");
  const validiumRateLimitAmountInWei = getRequiredEnvVar("VALIDIUM_RATE_LIMIT_AMOUNT");
  const validiumGenesisTimestamp = getRequiredEnvVar("L2_GENESIS_TIMESTAMP");

  // The default true preserves existing local deploy behavior; the quickstart opts into false to skip the gateway.
  const deployForcedTransactionGateway = getBooleanEnvVarOrDefault("DEPLOY_FORCED_TRANSACTION_GATEWAY", true);

  const validiumName = "ValidiumV3Stub";
  const validiumImplementationName = "ValidiumV3StubImplementation";
  const forcedTransactionGatewayName = "ForcedTransactionGateway";

  const pauseTypeRoles = getEnvVarOrDefault("VALIDIUM_PAUSE_TYPES_ROLES", VALIDIUM_PAUSE_TYPES_ROLES);
  const unpauseTypeRoles = getEnvVarOrDefault("VALIDIUM_UNPAUSE_TYPES_ROLES", VALIDIUM_UNPAUSE_TYPES_ROLES);

  // Use random hardcoded address until we introduce YieldManager E2E tests
  const automationServiceAddress = "0x3A9f0c2b8e7D4F6e1b5a9C2e0Fd7a4B6C8e9F1A2";
  const defaultRoleAddresses = [
    ...generateRoleAssignments(VALIDIUM_ROLES, validiumSecurityCouncil, [
      { role: OPERATOR_ROLE, addresses: validiumOperators },
    ]),
    { role: YIELD_PROVIDER_STAKING_ROLE, addressWithRole: automationServiceAddress },
  ];
  const roleAddresses = getEnvVarOrDefault("VALIDIUM_ROLE_ADDRESSES", defaultRoleAddresses);

  // No real RISC-V guest-program verifier keys exist yet. acceptDataRollingHash/finalizeBlocks are
  // no-op stubs on this contract, so a key is never actually checked against a proof - a randomly
  // generated placeholder is enough to satisfy initialization.
  const verifierKeys = [ethers.hexlify(ethers.randomBytes(32))];

  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);

  const wallet = new ethers.Wallet(process.env.DEPLOYER_PRIVATE_KEY!, provider);

  // The public quickstart can pin local L1 deploy gas for deterministic boot
  // behavior via L1_DEPLOY_GAS_PRICE_WEI; otherwise the provider's fee data is
  // used. resolveOneModelFeeOverrides guarantees a single, complete fee model.
  const feeOverrides = await resolveOneModelFeeOverrides(provider, "L1_DEPLOY_GAS_PRICE_WEI");

  const walletNonce = await getDeployNonceFromEnv(wallet, "L1_NONCE");

  const [validiumImplementation, proxyAdmin, addressFilter] = await Promise.all([
    deployContractFromArtifacts(validiumImplementationName, ValidiumV3StubAbi, ValidiumV3StubBytecode, wallet, {
      nonce: walletNonce,
      ...feeOverrides,
    }),
    deployContractFromArtifacts(ProxyAdminContractName, ProxyAdminAbi, ProxyAdminBytecode, wallet, {
      nonce: walletNonce + 1,
      ...feeOverrides,
    }),
    deployContractFromArtifacts(
      AddressFilterContractName,
      AddressFilterAbi,
      AddressFilterBytecode,
      wallet,
      validiumSecurityCouncil,
      PRECOMPILES_ADDRESSES,
      {
        nonce: walletNonce + 2,
        ...feeOverrides,
      },
    ),
  ]);

  const [proxyAdminAddress, validiumImplementationAddress, addressFilterAddress] = await Promise.all([
    proxyAdmin.getAddress(),
    validiumImplementation.getAddress(),
    addressFilter.getAddress(),
  ]);

  const initializer = getInitializerData(ValidiumV3StubAbi, "initialize", [
    {
      initialBlockHash: validiumInitialBlockHash,
      initialL2BlockNumber: validiumInitialL2BlockNumber,
      genesisTimestamp: validiumGenesisTimestamp,
      // finalizeBlocks is a no-op stub here, so the verifier is never invoked - DEAD_ADDRESS is a
      // non-zero placeholder that satisfies the zero-address check without deploying a real verifier.
      defaultVerifier: DEAD_ADDRESS,
      rateLimitPeriodInSeconds: validiumRateLimitPeriodInSeconds,
      rateLimitAmountInWei: validiumRateLimitAmountInWei,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
      verifierKeys,
      defaultAdmin: validiumSecurityCouncil,
      dataRollingHashProvider: ADDRESS_ZERO,
      addressFilter: addressFilterAddress,
    },
  ]);

  const validiumContract = await deployContractFromArtifacts(
    validiumName,
    TransparentUpgradeableProxyAbi,
    TransparentUpgradeableProxyBytecode,
    wallet,
    validiumImplementationAddress,
    proxyAdminAddress,
    initializer,
    {
      nonce: walletNonce + 3,
      ...feeOverrides,
    },
  );

  const validiumAddress = await validiumContract.getAddress();

  if (deployForcedTransactionGateway) {
    const destinationChainId = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID");
    const l2BlockBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER");
    const maxGasLimit = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT");
    const maxInputLengthBuffer = getRequiredEnvVar("FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER");
    const l2BlockDurationSeconds = getRequiredEnvVar("FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS");
    const blockNumberDeadlineBuffer = getRequiredEnvVar("FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER");
    const securityCouncilPrivateKey = getRequiredEnvVar("SECURITY_COUNCIL_PRIVATE_KEY");

    const mimc = await deployContractFromArtifacts(
      MimcAddressContractName,
      MimcAddressAbi,
      MimcAddressFilterBytecode,
      wallet,
      {
        nonce: walletNonce + 4,
        ...feeOverrides,
      },
    );
    const mimcAddress = await mimc.getAddress();

    const args = [
      validiumAddress,
      destinationChainId,
      l2BlockBuffer,
      maxGasLimit,
      maxInputLengthBuffer,
      validiumSecurityCouncil,
      addressFilterAddress,
      l2BlockDurationSeconds,
      blockNumberDeadlineBuffer,
    ];

    const forcedTransactionGateway = await deployContractFromArtifacts(
      forcedTransactionGatewayName,
      ForcedTransactionGatewayAbi,
      ForcedTransactionGatewayBytecode,
      wallet,
      { libraries: { "src/libraries/Mimc.sol:Mimc": mimcAddress } },
      ...args,
      {
        nonce: walletNonce + 5,
        ...feeOverrides,
      },
    );

    const forcedTransactionGatewayAddress = await forcedTransactionGateway.getAddress();
    const securityCouncilWallet = new ethers.Wallet(securityCouncilPrivateKey, provider);
    const validium = new ethers.Contract(validiumAddress, ValidiumV3StubAbi, securityCouncilWallet);

    console.log(
      `Granting FORCED_TRANSACTION_SENDER_ROLE to ForcedTransactionGateway at ${forcedTransactionGatewayAddress}...`,
    );
    const grantRoleTx = await validium.grantRole(
      FORCED_TRANSACTION_SENDER_ROLE,
      forcedTransactionGatewayAddress,
      feeOverrides,
    );
    await grantRoleTx.wait();
    console.log(`FORCED_TRANSACTION_SENDER_ROLE granted to ForcedTransactionGateway`);
  } else {
    console.log(
      "DEPLOY_FORCED_TRANSACTION_GATEWAY=false; skipping Mimc and ForcedTransactionGateway deploy. " +
        "The next L1 deploy starts after the Validium proxy nonce.",
    );
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
