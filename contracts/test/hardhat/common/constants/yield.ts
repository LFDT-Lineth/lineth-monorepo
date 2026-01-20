import { parseUnits } from "ethers";
import { ethers } from "hardhat";

export const MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS = 2000;
export const TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS = 2500;
export const MINIMUM_WITHDRAWAL_RESERVE_AMOUNT = ethers.parseEther("1000");
export const TARGET_WITHDRAWAL_RESERVE_AMOUNT = ethers.parseEther("1250");
export const MAX_BPS = 10000n;
export const CONNECT_DEPOSIT = ethers.parseEther("1");

// Values from constructor params for Lido PreDepositGuarantee.sol Hoodi deployment - https://hoodi.etherscan.io/address/0x8b289fc1af2bbc589f5990b94061d851c48683a3#code
export const GI_FIRST_VALIDATOR = "0x0000000000000000000000000000000000000000000000000096000000000028";
// gIndex = 2^depth + field_index
// field_index = 35, 36th element in Electra BeaconState - https://github.com/ethereum/consensus-specs/blob/5390b77256a9fd6c1ebe0c7e3f8a3da033476ddf/specs/electra/beacon-chain.md?plain=1#L417
// depth = ceil (log2 field_index_num) = 6
// 2^6 + 35 = 99
// Can check here with following excerpt - https://github.com/lidofinance/community-staking-module/blob/4d5f4700e356dc502c484456fbf924cba56206ad/script/gindex.mjs#L36
/**
  {
    const PendingPartialWithdrawals = Fork.BeaconState.getPathInfo(["pendingPartialWithdrawals"]).type;

    const gI = pack(
      Fork.BeaconState.getPathInfo(["pendingPartialWithdrawals"]).gindex,
      PendingPartialWithdrawals.limit,
    );

    console.log(`${fork}::gIPendingPartialWithdrawalsRoot:`, toBytes32String(gI));
  }
 */
export const GI_PENDING_PARTIAL_WITHDRAWALS_ROOT = "0x000000000000000000000000000000000000000000000000000000000000631b";

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
export const MAX_0X2_VALIDATOR_EFFECTIVE_BALANCE_GWEI = parseUnits("2048", "gwei");

export const BEACON_PROOF_WITNESS_TYPE =
  "tuple(uint64 childBlockTimestamp, uint64 proposerIndex, tuple(bytes32[] proof, uint64 effectiveBalance, uint64 activationEpoch, uint64 activationEligibilityEpoch) validatorContainerWitness, tuple(bytes32[] proof, tuple(uint64 validatorIndex, uint64 amount, uint64 withdrawableEpoch)[] pendingPartialWithdrawals) pendingPartialWithdrawalsWitness)";
