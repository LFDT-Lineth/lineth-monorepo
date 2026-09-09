export const MAX_FORCED_TRANSACTION_GAS_LIMIT = 300_000n;
export const MAX_INPUT_LENGTH_LIMIT = 1000n;
export const HARDHAT_CHAIN_ID = 31337n;
export const LINEA_MAINNET_CHAIN_ID = 59144n;
export const FORCED_TRANSACTION_FEE = 1_000n;
export const L2_BLOCK_DURATION_SECONDS = 1n;
export const BLOCK_NUMBER_DEADLINE_BUFFER = 60n;

// Conservative over-estimate of the worst-case Osaka intrinsic gas for a forced transaction
// with MAX_INPUT_LENGTH_LIMIT = 1 000 bytes and an empty access list:
//   21 000  base
// + 16 000  1 000 non-zero initcode bytes × 16 gas each
// + 32 000  contract-creation extra (to = address(0) path)
// +     64  initcode word charge: 2 × ceil(1 000 / 32) = 64
// --------
//   69 064  exact worst-case; 70 000 is the rounded-up deployment constant.
// If the network's calldata byte limit or per-byte gas costs change, update this value
// and the prover's RLP-byte-size configuration together.
export const MIN_FORCED_TRANSACTION_GAS_LIMIT = 70_000n;

// Minimum maxFeePerGas (base fee floor) accepted by the gateway for a forced transaction.
// Set to zero for gasless networks, which disables the zero-fee and base-fee-floor checks.
export const MINIMUM_BASE_GAS_FEE = 7_000_000_000n; // 7 gwei

// Permissive minimum base gas fee used only when deploying the gateway in tests.
// Static signed transaction fixtures (e.g. l2SendMessage.json, withCalldata.json) carry a
// maxFeePerGas of 0x4762da9 (74 853 801 wei ≈ 75 mwei), so the test gateway must accept
// values at least that low. This value must NOT be used for production deployments.
export const TEST_MINIMUM_BASE_GAS_FEE = 7n; // 7 wei — below all fixture maxFeePerGas values
