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
