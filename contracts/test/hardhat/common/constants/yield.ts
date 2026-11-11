import { parseEther, parseGwei } from "viem";

export const MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS = 2000;
export const TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS = 2500;
export const MINIMUM_WITHDRAWAL_RESERVE_AMOUNT = parseEther("1000");
export const TARGET_WITHDRAWAL_RESERVE_AMOUNT = parseEther("1250");
export const MAX_BPS = 10000n;
export const CONNECT_DEPOSIT = parseEther("1");

export { GI_FIRST_VALIDATOR, GI_PENDING_PARTIAL_WITHDRAWALS_ROOT } from "../../../../common/constants/yield.js";

// YieldProviderVendor enum
// export const UNUSED_YIELD_PROVIDER_VENDOR = 0;
// export const LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR = 1;

export const enum YieldProviderVendor {
  UNUSED_YIELD_PROVIDER_VENDOR = 0,
  LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR = 1,
}

// ProgressOssificationResult enum
export const enum ProgressOssificationResult {
  REINITIATED = 0,
  NOOP = 1,
  COMPLETE = 2,
}

export const enum OperationType {
  FUND_YIELD_PROVIDER = 0,
  REPORT_YIELD = 1,
}

export const FAR_FUTURE_EXIT_EPOCH = 18446744073709551615n;
export const SHARD_COMMITTEE_PERIOD = 256n;
export const SLOTS_PER_EPOCH = 32n;

export const THIRTY_TWO_ETH_IN_GWEI = 32000000000n;
export const MAX_0X2_VALIDATOR_EFFECTIVE_BALANCE_GWEI = parseGwei("2048");

export const BEACON_PROOF_WITNESS_TYPE =
  "tuple(uint64 childBlockTimestamp, uint64 proposerIndex, tuple(bytes32[] proof, uint64 effectiveBalance, uint64 activationEpoch, uint64 activationEligibilityEpoch) validatorContainerWitness, tuple(bytes32[] proof, tuple(uint64 validatorIndex, uint64 amount, uint64 withdrawableEpoch)[] pendingPartialWithdrawals) pendingPartialWithdrawalsWitness)";
