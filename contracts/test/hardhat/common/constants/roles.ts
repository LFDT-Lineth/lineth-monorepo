import { generateKeccak256 } from "../helpers";
import { HASH_ZERO } from "./general";

// Linea XP Token roles
export const MINTER_ROLE = generateKeccak256(["string"], ["MINTER_ROLE"], true);
export const TRANSFER_ROLE = generateKeccak256(["string"], ["TRANSFER_ROLE"], true);

// TimeLock roles
export const TIMELOCK_ADMIN_ROLE = generateKeccak256(["string"], ["TIMELOCK_ADMIN_ROLE"], true);
export const PROPOSER_ROLE = generateKeccak256(["string"], ["PROPOSER_ROLE"], true);
export const EXECUTOR_ROLE = generateKeccak256(["string"], ["EXECUTOR_ROLE"], true);
export const CANCELLER_ROLE = generateKeccak256(["string"], ["CANCELLER_ROLE"], true);

// Roles hashes
export const DEFAULT_ADMIN_ROLE = HASH_ZERO;
export const FUNCTION_EXECUTOR_ROLE = generateKeccak256(["string"], ["FUNCTION_EXECUTOR_ROLE"], true);
export const RATE_LIMIT_SETTER_ROLE = generateKeccak256(["string"], ["RATE_LIMIT_SETTER_ROLE"], true);
export const USED_RATE_LIMIT_RESETTER_ROLE = generateKeccak256(["string"], ["USED_RATE_LIMIT_RESETTER_ROLE"], true);
export const L1_L2_MESSAGE_SETTER_ROLE = generateKeccak256(["string"], ["L1_L2_MESSAGE_SETTER_ROLE"], true);
export const PAUSE_ALL_ROLE = generateKeccak256(["string"], ["PAUSE_ALL_ROLE"], true);
export const UNPAUSE_ALL_ROLE = generateKeccak256(["string"], ["UNPAUSE_ALL_ROLE"], true);
export const PAUSE_L1_L2_ROLE = generateKeccak256(["string"], ["PAUSE_L1_L2_ROLE"], true);
export const UNPAUSE_L1_L2_ROLE = generateKeccak256(["string"], ["UNPAUSE_L1_L2_ROLE"], true);
export const PAUSE_L2_L1_ROLE = generateKeccak256(["string"], ["PAUSE_L2_L1_ROLE"], true);
export const UNPAUSE_L2_L1_ROLE = generateKeccak256(["string"], ["UNPAUSE_L2_L1_ROLE"], true);
export const PAUSE_BLOB_SUBMISSION_ROLE = generateKeccak256(["string"], ["PAUSE_BLOB_SUBMISSION_ROLE"], true);
export const UNPAUSE_BLOB_SUBMISSION_ROLE = generateKeccak256(["string"], ["UNPAUSE_BLOB_SUBMISSION_ROLE"], true);
export const PAUSE_FINALIZATION_ROLE = generateKeccak256(["string"], ["PAUSE_FINALIZATION_ROLE"], true);
export const UNPAUSE_FINALIZATION_ROLE = generateKeccak256(["string"], ["UNPAUSE_FINALIZATION_ROLE"], true);
export const MINIMUM_FEE_SETTER_ROLE = generateKeccak256(["string"], ["MINIMUM_FEE_SETTER_ROLE"], true);
export const OPERATOR_ROLE = generateKeccak256(["string"], ["OPERATOR_ROLE"], true);
export const VERIFIER_SETTER_ROLE = generateKeccak256(["string"], ["VERIFIER_SETTER_ROLE"], true);
export const VERIFIER_UNSETTER_ROLE = generateKeccak256(["string"], ["VERIFIER_UNSETTER_ROLE"], true);
export const L1_MERKLE_ROOTS_SETTER_ROLE = generateKeccak256(["string"], ["L1_MERKLE_ROOTS_SETTER_ROLE"], true);
export const L2_MERKLE_ROOTS_SETTER_ROLE = generateKeccak256(["string"], ["L2_MERKLE_ROOTS_SETTER_ROLE"], true);
export const BAD_STARTING_HASH = generateKeccak256(["string"], ["BAD_STARTING_HASH"], true);

// TokenBridge roles
export const PAUSE_INITIATE_TOKEN_BRIDGING_ROLE = generateKeccak256(
  ["string"],
  ["PAUSE_INITIATE_TOKEN_BRIDGING_ROLE"],
  true,
);
export const PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = generateKeccak256(
  ["string"],
  ["PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE"],
  true,
);
export const UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE = generateKeccak256(
  ["string"],
  ["UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE"],
  true,
);
export const UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = generateKeccak256(
  ["string"],
  ["UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE"],
  true,
);
export const SET_RESERVED_TOKEN_ROLE = generateKeccak256(["string"], ["SET_RESERVED_TOKEN_ROLE"], true);
export const REMOVE_RESERVED_TOKEN_ROLE = generateKeccak256(["string"], ["REMOVE_RESERVED_TOKEN_ROLE"], true);
export const SET_CUSTOM_CONTRACT_ROLE = generateKeccak256(["string"], ["SET_CUSTOM_CONTRACT_ROLE"], true);
export const SET_MESSAGE_SERVICE_ROLE = generateKeccak256(["string"], ["SET_MESSAGE_SERVICE_ROLE"], true);
