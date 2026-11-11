import { type Address, pad, toHex } from "viem";

import { getAccountsFixture, getRoleAddressesFixture, getValidiumRoleAddressesFixture } from "./before.js";
import { PRECOMPILES_ADDRESSES } from "../../../../common/constants/index.js";
import {
  LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
} from "../../../../common/constants/pauseTypes.js";
import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json" with { type: "json" };
import {
  ADDRESS_ZERO,
  BLOCK_NUMBER_DEADLINE_BUFFER,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  FALLBACK_OPERATOR_ADDRESS,
  INITIAL_WITHDRAW_LIMIT,
  L2_BLOCK_DURATION_SECONDS,
  LINEA_MAINNET_CHAIN_ID,
  LINETH_ROLLUP_INITIALIZE_SIGNATURE,
  MAX_FORCED_TRANSACTION_GAS_LIMIT,
  MAX_INPUT_LENGTH_LIMIT,
  ONE_DAY_IN_SECONDS,
  THREE_DAYS_IN_SECONDS,
  VALIDIUM_INITIALIZE_SIGNATURE,
} from "../../common/constants/index.js";
import { deployFromFactory, deployUpgradableFromFactory } from "../../common/deployment.js";
import { address, hex } from "../../common/helpers/index.js";

import type { LinethRollupInitializationData } from "../../common/types.js";

export async function deployRevertingVerifier(scenario: number): Promise<Address> {
  const verifier = await deployFromFactory("RevertingVerifier", [scenario]);
  return verifier.address;
}

export async function deployCallForwardingProxy(target: Address) {
  return deployFromFactory("CallForwardingProxy", [target]);
}

// Fixtures call the fixtures they build on directly instead of through `loadFixture`: Hardhat 3's network helpers
// lose snapshot invalidations made by a `loadFixture` nested inside another fixture (the outer call keeps the snapshot
// list it read before running the fixture), which later fails with HHE60013 when a stale snapshot is restored.
export async function deployValidiumFixture() {
  const { securityCouncil, nonAuthorizedAccount } = await getAccountsFixture();
  const roleAddresses = await getValidiumRoleAddressesFixture();

  const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

  const verifier = await deployTestPlonkVerifierForDataAggregation();
  const { parentStateRootHash } = firstCompressedDataContent;

  const initializationData = {
    initialStateRootHash: hex(parentStateRootHash),
    initialL2BlockNumber: 0,
    genesisTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
    defaultVerifier: verifier,
    rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
    rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
    roleAddresses,
    pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
    defaultAdmin: securityCouncil.address,
    shnarfProvider: ADDRESS_ZERO,
    addressFilter: addressFilter.address,
  };

  const validium = await deployUpgradableFromFactory("TestValidium", [initializationData], {
    initializer: VALIDIUM_INITIALIZE_SIGNATURE,
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
  });

  return { verifier, validium, addressFilter };
}

export async function deployMockYieldManager(): Promise<Address> {
  const mockYieldManager = await deployFromFactory("MockYieldManager");
  return mockYieldManager.address;
}

export async function deployLinethRollupFixture() {
  const { securityCouncil, nonAuthorizedAccount } = await getAccountsFixture();
  const roleAddresses = await getRoleAddressesFixture();

  const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

  const verifier = await deployTestPlonkVerifierForDataAggregation();
  const { parentStateRootHash } = firstCompressedDataContent;

  const yieldManager = await deployMockYieldManager();

  const initializationData: LinethRollupInitializationData = {
    initialStateRootHash: hex(parentStateRootHash),
    initialL2BlockNumber: 0n,
    genesisTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
    defaultVerifier: verifier,
    rateLimitPeriodInSeconds: BigInt(ONE_DAY_IN_SECONDS),
    rateLimitAmountInWei: BigInt(INITIAL_WITHDRAW_LIMIT),
    roleAddresses,
    pauseTypeRoles: LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
    unpauseTypeRoles: LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
    defaultAdmin: securityCouncil.address,
    shnarfProvider: ADDRESS_ZERO,
    addressFilter: addressFilter.address,
  };

  const linethRollup = await deployUpgradableFromFactory(
    "TestLinethRollup",
    [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
    {
      initializer: LINETH_ROLLUP_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order"],
    },
  );

  return { verifier, linethRollup, addressFilter, yieldManager, linethRollupInitializationData: initializationData };
}

export async function deployAddressFilter(securityCouncil: Address, nonAuthorizedAccount: Address[]) {
  const addressFilter = await deployFromFactory("AddressFilter", [
    securityCouncil,
    [...PRECOMPILES_ADDRESSES.map(address), ...nonAuthorizedAccount],
  ]);

  return { addressFilter };
}

export async function deployMimcFixture() {
  const mimc = await deployFromFactory("Mimc");
  return { mimc };
}

export async function deployForcedTransactionGatewayFixture() {
  const { securityCouncil } = await getAccountsFixture();
  const { linethRollup, addressFilter, verifier, yieldManager, linethRollupInitializationData } =
    await deployLinethRollupFixture();
  const { mimc } = await deployMimcFixture();

  const forcedTransactionGateway = await deployFromFactory(
    "ForcedTransactionGateway",
    [
      linethRollup.address,
      LINEA_MAINNET_CHAIN_ID,
      BigInt(THREE_DAYS_IN_SECONDS),
      MAX_FORCED_TRANSACTION_GAS_LIMIT,
      MAX_INPUT_LENGTH_LIMIT,
      securityCouncil.address,
      addressFilter.address,
      L2_BLOCK_DURATION_SECONDS,
      BLOCK_NUMBER_DEADLINE_BUFFER,
    ],
    { libraries: { Mimc: mimc.address } },
  );

  return {
    linethRollup,
    forcedTransactionGateway,
    addressFilter,
    mimc,
    verifier,
    yieldManager,
    linethRollupInitializationData,
  };
}

export async function deployAddressFilterFixture() {
  const { securityCouncil, nonAuthorizedAccount } = await getAccountsFixture();
  const { addressFilter } = await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);
  return { addressFilter };
}

async function deployTestPlonkVerifierForDataAggregation(): Promise<Address> {
  const mimc = await deployFromFactory("Mimc");
  const verifier = await deployFromFactory(
    "TestPlonkVerifierForDataAggregation",
    [
      [
        {
          value: toHex(59144, { size: 32 }),
          name: "chainId",
        },
        {
          value: toHex(7n, { size: 32 }),
          name: "baseFee",
        },
        {
          value: pad("0x8f81e2e3f8b46467523463835f965ffe476e1c9e", { size: 32 }),
          name: "coinbase",
        },
        {
          value: pad("0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec", { size: 32 }),
          name: "l2MessageServiceAddress",
        },
        {
          value: toHex(0n, { size: 32 }),
          name: "isAllowedCircuitID",
        },
      ],
    ],
    { libraries: { Mimc: mimc.address } },
  );
  return verifier.address;
}
