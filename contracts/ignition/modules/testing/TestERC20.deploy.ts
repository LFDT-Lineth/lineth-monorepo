import { parseEther } from "viem";

import { getRequiredEnvVar } from "../../../common/helpers/environment.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `11_deploy_TestERC20.ts`: `TEST_ERC20_INITIAL_SUPPLY` is in ether, passed to the contract in wei. */
export function testERC20Parameters(): ModuleParameters {
  const tokenName = getRequiredEnvVar("TEST_ERC20_NAME");
  const tokenSymbol = getRequiredEnvVar("TEST_ERC20_SYMBOL");
  const initialSupply = getRequiredEnvVar("TEST_ERC20_INITIAL_SUPPLY");
  return { name: tokenName, symbol: tokenSymbol, initialSupply: parseEther(initialSupply) };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async () => ({
  TestERC20: testERC20Parameters(),
});
