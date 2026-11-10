import { network } from "hardhat";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

const tokenNames = ["L1USDT", "L1DAI", "L1WETH", "L2UNI", "L2SHIBA"];

export async function deployTokens(verbose = false) {
  const { viem } = await network.getOrCreate();
  const tokens: { [name: string]: ContractReturnType<"MockERC20MintBurn"> } = {};

  for (const name of tokenNames) {
    tokens[name] = await viem.deployContract("MockERC20MintBurn", [name, name]);
    if (verbose) {
      console.log(name, "deployed");
    }
  }

  return tokens;
}
