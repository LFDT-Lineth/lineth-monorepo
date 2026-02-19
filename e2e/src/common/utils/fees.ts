import { MAX_FEE_PER_GAS_FALLBACK, MAX_PRIORITY_FEE_PER_GAS_FALLBACK } from "../constants";

export type Eip1559Fees = {
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
};

export function normalizeEip1559Fees(maxPriorityFeePerGas: bigint, maxFeePerGas: bigint): Eip1559Fees {
  const resolvedMaxPriorityFeePerGas = maxPriorityFeePerGas || MAX_PRIORITY_FEE_PER_GAS_FALLBACK;
  const resolvedMaxFeePerGas = maxFeePerGas || MAX_FEE_PER_GAS_FALLBACK;

  return {
    maxPriorityFeePerGas: resolvedMaxPriorityFeePerGas,
    maxFeePerGas:
      resolvedMaxFeePerGas >= resolvedMaxPriorityFeePerGas ? resolvedMaxFeePerGas : resolvedMaxPriorityFeePerGas,
  };
}
