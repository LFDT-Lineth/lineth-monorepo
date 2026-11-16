import { parseEther } from "viem";

import { getRequiredEnvVar } from "../../../common/helpers/environment.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** `TEST_ERC20_INITIAL_SUPPLY` is in ether, passed to the contract in wei. */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async () => {
  const tokenName = getRequiredEnvVar("TEST_ERC20_NAME");
  const tokenSymbol = getRequiredEnvVar("TEST_ERC20_SYMBOL");
  const initialSupply = getRequiredEnvVar("TEST_ERC20_INITIAL_SUPPLY");
  return { TestERC20: { name: tokenName, symbol: tokenSymbol, initialSupply: parseEther(initialSupply) } };
};
