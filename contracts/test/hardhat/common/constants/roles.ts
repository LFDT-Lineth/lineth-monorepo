import { HASH_ZERO } from "./general.js";
import { generateKeccak256ForString } from "../helpers/hashing.js";

// TimeLock roles
export const TIMELOCK_ADMIN_ROLE = generateKeccak256ForString("TIMELOCK_ADMIN_ROLE");
export const PROPOSER_ROLE = generateKeccak256ForString("PROPOSER_ROLE");
export const EXECUTOR_ROLE = generateKeccak256ForString("EXECUTOR_ROLE");
export const CANCELLER_ROLE = generateKeccak256ForString("CANCELLER_ROLE");

// Roles hashes
export const DEFAULT_ADMIN_ROLE = HASH_ZERO;
export const FORCED_TRANSACTION_SENDER_ROLE = generateKeccak256ForString("FORCED_TRANSACTION_SENDER_ROLE");
export const FUNCTION_EXECUTOR_ROLE = generateKeccak256ForString("FUNCTION_EXECUTOR_ROLE");
export const RATE_LIMIT_SETTER_ROLE = generateKeccak256ForString("RATE_LIMIT_SETTER_ROLE");
export const USED_RATE_LIMIT_RESETTER_ROLE = generateKeccak256ForString("USED_RATE_LIMIT_RESETTER_ROLE");
export const L1_L2_MESSAGE_SETTER_ROLE = generateKeccak256ForString("L1_L2_MESSAGE_SETTER_ROLE");
export const PAUSE_ALL_ROLE = generateKeccak256ForString("PAUSE_ALL_ROLE");
export const UNPAUSE_ALL_ROLE = generateKeccak256ForString("UNPAUSE_ALL_ROLE");
export const PAUSE_L1_L2_ROLE = generateKeccak256ForString("PAUSE_L1_L2_ROLE");
export const UNPAUSE_L1_L2_ROLE = generateKeccak256ForString("UNPAUSE_L1_L2_ROLE");
export const PAUSE_L2_L1_ROLE = generateKeccak256ForString("PAUSE_L2_L1_ROLE");
export const UNPAUSE_L2_L1_ROLE = generateKeccak256ForString("UNPAUSE_L2_L1_ROLE");
export const PAUSE_STATE_DATA_SUBMISSION_ROLE = generateKeccak256ForString("PAUSE_STATE_DATA_SUBMISSION_ROLE");
export const UNPAUSE_STATE_DATA_SUBMISSION_ROLE = generateKeccak256ForString("UNPAUSE_STATE_DATA_SUBMISSION_ROLE");
export const PAUSE_FINALIZATION_ROLE = generateKeccak256ForString("PAUSE_FINALIZATION_ROLE");
export const UNPAUSE_FINALIZATION_ROLE = generateKeccak256ForString("UNPAUSE_FINALIZATION_ROLE");
export const MINIMUM_FEE_SETTER_ROLE = generateKeccak256ForString("MINIMUM_FEE_SETTER_ROLE");
export const OPERATOR_ROLE = generateKeccak256ForString("OPERATOR_ROLE");
export const VERIFIER_SETTER_ROLE = generateKeccak256ForString("VERIFIER_SETTER_ROLE");
export const VERIFIER_UNSETTER_ROLE = generateKeccak256ForString("VERIFIER_UNSETTER_ROLE");
export const SET_ADDRESS_FILTER_ROLE = generateKeccak256ForString("SET_ADDRESS_FILTER_ROLE");
export const L1_MERKLE_ROOTS_SETTER_ROLE = generateKeccak256ForString("L1_MERKLE_ROOTS_SETTER_ROLE");
export const L2_MERKLE_ROOTS_SETTER_ROLE = generateKeccak256ForString("L2_MERKLE_ROOTS_SETTER_ROLE");
export const SECURITY_COUNCIL_ROLE = generateKeccak256ForString("SECURITY_COUNCIL_ROLE");
// TokenBridge roles
export const PAUSE_INITIATE_TOKEN_BRIDGING_ROLE = generateKeccak256ForString("PAUSE_INITIATE_TOKEN_BRIDGING_ROLE");
export const PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = generateKeccak256ForString("PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE");
export const UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE = generateKeccak256ForString("UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE");
export const UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = generateKeccak256ForString("UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE");
export const SET_RESERVED_TOKEN_ROLE = generateKeccak256ForString("SET_RESERVED_TOKEN_ROLE");
export const REMOVE_RESERVED_TOKEN_ROLE = generateKeccak256ForString("REMOVE_RESERVED_TOKEN_ROLE");
export const SET_CUSTOM_CONTRACT_ROLE = generateKeccak256ForString("SET_CUSTOM_CONTRACT_ROLE");
export const SET_MESSAGE_SERVICE_ROLE = generateKeccak256ForString("SET_MESSAGE_SERVICE_ROLE");

export const SET_YIELD_MANAGER_ROLE = generateKeccak256ForString("SET_YIELD_MANAGER_ROLE");
export const YIELD_PROVIDER_STAKING_ROLE = generateKeccak256ForString("YIELD_PROVIDER_STAKING_ROLE");

// YieldManager related pause roles
export const PAUSE_NATIVE_YIELD_STAKING_ROLE = generateKeccak256ForString("PAUSE_NATIVE_YIELD_STAKING_ROLE");
export const UNPAUSE_NATIVE_YIELD_STAKING_ROLE = generateKeccak256ForString("UNPAUSE_NATIVE_YIELD_STAKING_ROLE");
export const PAUSE_NATIVE_YIELD_UNSTAKING_ROLE = generateKeccak256ForString("PAUSE_NATIVE_YIELD_UNSTAKING_ROLE");
export const UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE = generateKeccak256ForString("UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE");
export const PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE = generateKeccak256ForString(
  "PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE",
);
export const UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE = generateKeccak256ForString(
  "UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE",
);
export const PAUSE_NATIVE_YIELD_REPORTING_ROLE = generateKeccak256ForString("PAUSE_NATIVE_YIELD_REPORTING_ROLE");
export const UNPAUSE_NATIVE_YIELD_REPORTING_ROLE = generateKeccak256ForString("UNPAUSE_NATIVE_YIELD_REPORTING_ROLE");
