import type { ethers } from "ethers";

/**
 * Ethers-only fee helper for the Hardhat-free scripts (`local-deployments-artifacts/*`,
 * `common/helpers/feeOverrides.ts`). Hardhat-side code uses viem's `estimateFeesPerGas` instead.
 *
 * @param provider ethers JsonRpcProvider instance
 * @returns {Promise<{maxPriorityFeePerGas: *, maxFeePerGas: *}>}
 */
async function get1559Fees(
  provider: ethers.Provider,
): Promise<{ maxPriorityFeePerGas?: bigint; maxFeePerGas?: bigint; gasPrice?: bigint }> {
  const { maxPriorityFeePerGas, maxFeePerGas, gasPrice } = await provider.getFeeData();
  return {
    ...(maxPriorityFeePerGas ? { maxPriorityFeePerGas } : {}),
    ...(maxFeePerGas ? { maxFeePerGas } : {}),
    ...(gasPrice ? { gasPrice } : {}),
  };
}

export { get1559Fees };
