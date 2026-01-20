// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { PendingPartialWithdrawal } from "../libs/vendor/lido/BeaconTypes.sol";

/**
 * @title Interface for ValidatorContainerProofVerifier.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IValidatorContainerProofVerifier {
  /**
   * @notice Input for validator container proof verification
   * @custom:proof array of merkle proofs from Validator container root to Beacon block root
   * @custom:effectiveBalance of validator in CL state tree
   * @custom:activationEpoch of validator in CL state tree
   * @custom:activationEligibilityEpoch of validator in CL state tree
   */
  struct ValidatorContainerWitness {
    bytes32[] proof;
    uint64 effectiveBalance;
    uint64 activationEpoch;
    uint64 activationEligibilityEpoch;
  }

  /**
   * @notice Input for validator container proof verification
   * @custom:proof array of merkle proofs from pending_partial_withdrawals root to Beacon block root
   * @custom:pendingPartialWithdrawals Entire array of pending_partials_withdrawals
   */
  struct PendingPartialWithdrawalsWitness {
    bytes32[] proof;
    PendingPartialWithdrawal[] pendingPartialWithdrawals;
  }

  /**
   * @notice Input for validator container proof verification
   * @custom:childBlockTimestamp of EL block that has parent block beacon root in BEACON_ROOTS contract
   * @custom:proposerIndex of the beacon block for which the proof is generated
   */
  struct BeaconProofWitness {
    uint64 childBlockTimestamp;
    uint64 proposerIndex;
    ValidatorContainerWitness validatorContainerWitness;
    PendingPartialWithdrawalsWitness pendingPartialWithdrawalsWitness;
  }

  /// @notice Thrown when the slot and proposer index branch does not align with the supplied Merkle proof.
  error InvalidSlot();

  /// @notice Thrown when no beacon block root is found for the supplied child block timestamp in the EIP-4788 contract.
  error RootNotFound();

  /// @notice Thrown when the validator has not been active for the full `SHARD_COMMITTEE_PERIOD`.
  error ValidatorNotActiveForLongEnough();

  /**
   * @notice validates proof of validator container in CL against Beacon block root
   * @param _witness object containing user input passed as calldata
   * @param _pubkey The pubkey of validator to verify proof for.
   * @param _withdrawalCredentials The withdrawal credentials to verify proof with.
   * @param _validatorIndex Validator index for validator to withdraw from.
   * @param _slot Slot of the beacon block for which the proof is generated.
   * @param _childBlockTimestamp Timestamp of EL block that has parent block beacon root in BEACON_ROOTS contract.
   * @param _proposerIndex The proposer index of the beacon block for which the proof is generated.
   * @dev Reverts with `InvalidSlot` if slot/proposerIndex don't match the proof.
   * @dev Reverts with `ValidatorNotActiveForLongEnough` if validator hasn't been active for SHARD_COMMITTEE_PERIOD epochs.
   * @dev Reverts with `RootNotFound` if timestamp is not found in BEACON_ROOTS contract.
   * @dev Reverts with `InvalidProof`, `BranchHasExtraItem`, or `BranchHasMissingItem` if proof verification fails.
   */
  function verifyActiveValidatorContainer(
    IValidatorContainerProofVerifier.ValidatorContainerWitness calldata _witness,
    bytes calldata _pubkey,
    bytes32 _withdrawalCredentials,
    uint64 _validatorIndex,
    uint64 _slot,
    uint64 _childBlockTimestamp,
    uint64 _proposerIndex
  ) external view;

  /**
   * @notice validates proof of pending partial withdrawals in CL against Beacon block root
   * @param _witness object containing user input passed as calldata
   * @param _slot Slot of the beacon block for which the proof is generated.
   * @param _childBlockTimestamp Timestamp of EL block that has parent block beacon root in BEACON_ROOTS contract.
   * @param _proposerIndex The proposer index of the beacon block for which the proof is generated.
   * @dev Reverts with `InvalidSlot` if slot/proposerIndex don't match the proof.
   * @dev Reverts with `RootNotFound` if timestamp is not found in BEACON_ROOTS contract.
   * @dev Reverts with `InvalidProof`, `BranchHasExtraItem`, or `BranchHasMissingItem` if proof verification fails.
   */
  function verifyPendingPartialWithdrawals(
    PendingPartialWithdrawalsWitness calldata _witness,
    uint64 _slot,
    uint64 _childBlockTimestamp,
    uint64 _proposerIndex
  ) external view;
}
