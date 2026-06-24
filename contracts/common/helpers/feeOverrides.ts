import { ethers } from "ethers";

import { get1559Fees } from "../../scripts/utils";

// A transaction fee override that must describe exactly one fee model: either a
// legacy `gasPrice`, or a complete EIP-1559 pair, never a mix of both.
export type FeeOverrides = {
  gasPrice?: bigint;
  maxFeePerGas?: bigint;
  maxPriorityFeePerGas?: bigint;
};

// Parse an explicit wei gas-price env var. Returns undefined when unset/empty.
export function parseGasPriceWeiEnv(name: string): bigint | undefined {
  const raw = process.env[name];
  if (raw === undefined || raw === "") {
    return undefined;
  }
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${name} must be an integer wei value, got: ${raw}`);
  }
  return BigInt(raw);
}

// Reject overrides that mix legacy gasPrice with EIP-1559 fields, that send
// partial EIP-1559 data, or that set no fee at all. Ethers would otherwise
// silently drop or combine conflicting fields.
export function assertSingleFeeModel(fees: FeeOverrides): FeeOverrides {
  const hasLegacy = fees.gasPrice !== undefined;
  const hasMaxFee = fees.maxFeePerGas !== undefined;
  const hasPriorityFee = fees.maxPriorityFeePerGas !== undefined;

  if (hasLegacy && (hasMaxFee || hasPriorityFee)) {
    throw new Error("deploy fee overrides must not set both legacy gasPrice and EIP-1559 fields");
  }
  if ((hasMaxFee || hasPriorityFee) && !(hasMaxFee && hasPriorityFee)) {
    throw new Error("EIP-1559 deploy fee overrides require both maxFeePerGas and maxPriorityFeePerGas");
  }
  if (!hasLegacy && !hasMaxFee && !hasPriorityFee) {
    throw new Error("deploy fee overrides must set a legacy gasPrice or complete EIP-1559 fields");
  }
  return fees;
}

// Effective per-gas price used to size a value+gas budget: the legacy gasPrice
// when legacy fees are selected, otherwise the EIP-1559 ceiling maxFeePerGas.
export function feeBudgetPricePerGas(fees: FeeOverrides): bigint {
  assertSingleFeeModel(fees);
  if (fees.gasPrice !== undefined) {
    return fees.gasPrice;
  }
  // assertSingleFeeModel guarantees maxFeePerGas is present here.
  return fees.maxFeePerGas as bigint;
}

// Resolve a single-model fee override for a deploy transaction:
//   - an explicit wei gas-price env var wins and forces a legacy gasPrice;
//   - otherwise prefer a complete EIP-1559 pair from the provider;
//   - otherwise fall back to the provider's legacy gasPrice;
//   - throw when the provider returns only partial or no usable fee data.
export async function resolveOneModelFeeOverrides(
  provider: ethers.Provider,
  explicitGasPriceEnvName?: string,
): Promise<FeeOverrides> {
  if (explicitGasPriceEnvName !== undefined) {
    const explicitGasPrice = parseGasPriceWeiEnv(explicitGasPriceEnvName);
    if (explicitGasPrice !== undefined) {
      console.log(`Using environment variable ${explicitGasPriceEnvName}=${explicitGasPrice.toString()}`);
      return assertSingleFeeModel({ gasPrice: explicitGasPrice });
    }
  }

  const { gasPrice, maxFeePerGas, maxPriorityFeePerGas } = await get1559Fees(provider);
  if (maxFeePerGas !== undefined && maxPriorityFeePerGas !== undefined) {
    return assertSingleFeeModel({ maxFeePerGas, maxPriorityFeePerGas });
  }
  if (gasPrice !== undefined) {
    return assertSingleFeeModel({ gasPrice });
  }

  const hint = explicitGasPriceEnvName ? ` Set ${explicitGasPriceEnvName} explicitly.` : "";
  if (maxFeePerGas !== undefined || maxPriorityFeePerGas !== undefined) {
    throw new Error(`Provider returned incomplete EIP-1559 deploy fee data.${hint}`);
  }
  throw new Error(`Provider returned no usable deploy fee data.${hint}`);
}
