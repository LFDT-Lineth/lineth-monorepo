import type { Address, Hex } from "viem";

export interface YieldManagerInitializationData {
  pauseTypeRoles: { pauseType: number; role: Hex }[];
  unpauseTypeRoles: { pauseType: number; role: Hex }[];
  roleAddresses: { addressWithRole: Address; role: Hex }[];
  initialL2YieldRecipients: Address[];
  defaultAdmin: Address;
  initialMinimumWithdrawalReservePercentageBps: number; // uint16 on-chain
  initialTargetWithdrawalReservePercentageBps: number; // uint16 on-chain
  initialMinimumWithdrawalReserveAmount: bigint; // uint256 on-chain
  initialTargetWithdrawalReserveAmount: bigint; // uint256 on-chain
}

export interface YieldProviderRegistration {
  yieldProviderVendor: number;
  primaryEntrypoint: Address;
  ossifiedEntrypoint: Address;
  usersFundsIncrement: bigint;
}

export interface ValidatorContainer {
  pubkey: Hex;
  withdrawalCredentials: Hex;
  effectiveBalance: bigint;
  slashed: boolean;
  activationEligibilityEpoch: bigint;
  activationEpoch: bigint;
  exitEpoch: bigint;
  withdrawableEpoch: bigint;
}

export interface BeaconBlockHeader {
  slot: bigint;
  proposerIndex: bigint;
  parentRoot: Hex;
  stateRoot: Hex;
  bodyRoot: Hex;
}

export interface PendingPartialWithdrawal {
  validatorIndex: bigint;
  amount: bigint;
  withdrawableEpoch: bigint;
}

export interface ValidatorContainerWitness {
  proof: Hex[];
  effectiveBalance: bigint;
  activationEpoch: bigint;
  activationEligibilityEpoch: bigint;
}

export interface PendingPartialWithdrawalsWitness {
  proof: Hex[];
  pendingPartialWithdrawals: PendingPartialWithdrawal[];
}

export interface BeaconProofWitness {
  childBlockTimestamp: bigint;
  proposerIndex: bigint;
  validatorContainerWitness: ValidatorContainerWitness;
  pendingPartialWithdrawalsWitness: PendingPartialWithdrawalsWitness;
}

export interface EIP4788Witness {
  // Beacon block root
  blockRoot: Hex;
  // GI First Validator
  gIFirstValidator: Hex;
  beaconBlockHeader: BeaconBlockHeader;
  validatorIndex: bigint;
  pubkey: Hex;
  withdrawalCredentials: Hex;
  beaconProofWitness: BeaconProofWitness;
}

export interface ClaimMessageWithProofParams {
  proof: Hex[];
  messageNumber: bigint;
  leafIndex: number; // uint32 on-chain
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  feeRecipient: Address;
  merkleRoot: Hex;
  data: Hex;
}
