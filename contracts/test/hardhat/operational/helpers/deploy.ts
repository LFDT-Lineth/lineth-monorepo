import { type Address, getAddress, parseUnits } from "viem";

import { deployFromFactory, deployUpgradableFromFactory } from "../../common/deployment.js";
import { getConnection } from "../../common/helpers/connection.js";
import { ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE } from "../constants.js";
import { getRollupRevenueVaultAccountsFixture } from "./before.js";
import {
  L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  L2_MESSAGE_SERVICE_ROLES,
  L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
} from "../../../../common/constants/index.js";
import { generateRoleAssignments } from "../../../../common/helpers/roles.js";
import { deployTokenBridge } from "../../../../scripts/tokenBridge/test/deployTokenBridges.js";
import { INITIAL_WITHDRAW_LIMIT, L1_L2_MESSAGE_SETTER_ROLE, ONE_DAY_IN_SECONDS } from "../../common/constants/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

export async function deployWETH9Fixture(): Promise<Address> {
  const weth9 = await deployFromFactory("TestWETH9");
  // Checksummed, like ethers' `getAddress()`.
  return getAddress(weth9.address);
}

async function deployL2MessageService(adminAddress: Address, l1l2MessageSetterAddress: Address) {
  const roleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, adminAddress, [
    { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l1l2MessageSetterAddress] },
  ]);

  return deployUpgradableFromFactory("L2MessageService", [
    ONE_DAY_IN_SECONDS,
    INITIAL_WITHDRAW_LIMIT,
    adminAddress,
    roleAddresses,
    L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
    L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
  ]);
}

export async function deployTestDexSwapAdapterFixture(
  lineaTokenAddress: Address,
): Promise<ContractReturnType<"TestDexSwapAdapter">> {
  const testWETH9 = await deployWETH9Fixture();
  const router = await deployFromFactory("TestDexRouter");
  return deployFromFactory("TestDexSwapAdapter", [router.address, testWETH9, lineaTokenAddress, 50]);
}

export async function deployRollupRevenueVaultFixture() {
  const { networkHelpers } = await getConnection();
  const { admin, invoiceSubmitter, burner, invoicePaymentReceiver, l1LineaTokenBurner, l1l2MessageSetter } =
    await getRollupRevenueVaultAccountsFixture();

  const messageService = await deployL2MessageService(admin.address, l1l2MessageSetter.address);
  const { l2TokenBridge: tokenBridge } = await deployTokenBridge(messageService.address, false);
  const l2LineaToken = await deployFromFactory("TestERC20", ["TestERC20", "TEST", parseUnits("1000000000", 18)]);
  const dexSwapAdapter = await deployTestDexSwapAdapterFixture(l2LineaToken.address);

  const rollupRevenueVault = await deployUpgradableFromFactory(
    "RollupRevenueVault",
    [
      BigInt(await networkHelpers.time.latest()) - BigInt(ONE_DAY_IN_SECONDS),
      admin.address,
      invoiceSubmitter.address,
      burner.address,
      invoicePaymentReceiver.address,
      tokenBridge.address,
      messageService.address,
      l1LineaTokenBurner.address,
      l2LineaToken.address,
      dexSwapAdapter.address,
    ],
    {
      initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order"],
    },
  );

  return { rollupRevenueVault, l2LineaToken, tokenBridge, l1LineaTokenBurner, messageService, dexSwapAdapter };
}
