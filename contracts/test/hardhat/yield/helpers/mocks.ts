import { type Address, type Hex, encodeAbiParameters, parseAbiParameters } from "viem";
import { generatePrivateKey, privateKeyToAddress } from "viem/accounts";

import { deployMockWithdrawTarget, deployMockYieldProvider } from "./deploy.js";
import { EMPTY_CALLDATA } from "../../common/constants/index.js";
import { YieldProviderVendor } from "../../common/constants/index.js";
import { getAccountsFixture } from "../../common/helpers/index.js";

import type { YieldProviderRegistration } from "./types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

const randomAddress = (): Address => privateKeyToAddress(generatePrivateKey());

export const buildMockYieldProviderRegistration = (
  overrides: Partial<{
    yieldProviderVendor: number;
    primaryEntrypoint: Address;
    ossifiedEntrypoint: Address;
    usersFundsIncrement: bigint;
  }> = {},
): YieldProviderRegistration => ({
  yieldProviderVendor:
    (overrides.yieldProviderVendor ?? Math.random() < 0.5)
      ? YieldProviderVendor.UNUSED_YIELD_PROVIDER_VENDOR
      : YieldProviderVendor.LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR,
  primaryEntrypoint: overrides.primaryEntrypoint ?? randomAddress(),
  ossifiedEntrypoint: overrides.ossifiedEntrypoint ?? randomAddress(),
  usersFundsIncrement: overrides.usersFundsIncrement ?? 0n,
});

export const buildVendorExitData = (
  overrides: Partial<{
    newVaultOwner: Address;
  }> = {},
): Hex => {
  const params = {
    newVaultOwner: overrides.newVaultOwner ?? randomAddress(),
  };

  return encodeAbiParameters(parseAbiParameters("address"), [params.newVaultOwner]);
};

export const buildVendorInitializationData = (
  overrides: Partial<{
    defaultAdmin: Address;
    nodeOperator: Address;
    nodeOperatorManager: Address;
    nodeOperatorFeeBP: bigint;
    confirmExpiry: bigint;
    roleAssignments: { addressWithRole: Address; role: Hex }[];
  }> = {},
): Hex => {
  const params = {
    defaultAdmin: overrides.defaultAdmin ?? randomAddress(),
    nodeOperator: overrides.nodeOperator ?? randomAddress(),
    nodeOperatorManager: overrides.nodeOperatorManager ?? randomAddress(),
    nodeOperatorFeeBP: overrides.nodeOperatorFeeBP ?? 0n,
    confirmExpiry: overrides.confirmExpiry ?? 0n,
    roleAssignments: overrides.roleAssignments ?? [],
  };

  return encodeAbiParameters(
    parseAbiParameters("address, address, address, uint256, uint256, (address addressWithRole, bytes32 role)[]"),
    [
      params.defaultAdmin,
      params.nodeOperator,
      params.nodeOperatorManager,
      params.nodeOperatorFeeBP,
      params.confirmExpiry,
      params.roleAssignments,
    ],
  );
};

export const addMockYieldProvider = async (yieldManager: ContractReturnType<"TestYieldManager">) => {
  const { securityCouncil } = await getAccountsFixture();
  const mockYieldProvider = await deployMockYieldProvider();
  const mockYieldProviderAddress = mockYieldProvider.address;
  const mockRegistration = buildMockYieldProviderRegistration();
  await yieldManager.write.addYieldProvider([mockYieldProviderAddress, EMPTY_CALLDATA], {
    account: securityCouncil.account,
  });
  const mockWithdrawTarget = await deployMockWithdrawTarget();
  const mockWithdrawTargetAddress = mockWithdrawTarget.address;
  await yieldManager.write.setMockWithdrawTarget([mockYieldProviderAddress, mockWithdrawTargetAddress], {
    account: securityCouncil.account,
  });
  return { mockWithdrawTarget, mockYieldProvider, mockYieldProviderAddress, mockRegistration };
};
