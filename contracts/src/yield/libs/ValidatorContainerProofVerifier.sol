// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.33;
import { GIndex, pack, concat } from "./vendor/lido/GIndex.sol";
import { SSZ } from "./vendor/lido/SSZ.sol";
import { Validator } from "./vendor/lido/BeaconTypes.sol";
import { IValidatorContainerProofVerifier } from "../interfaces/IValidatorContainerProofVerifier.sol";
import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";
import { ErrorUtils } from "../../libraries/ErrorUtils.sol";

/**
 * @title ValidatorContainerProofVerifier
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 * @notice Verifies merkle proofs for Validator container against the EIP-4788 beacon root.
 *
 * Modified version of CLProofVerifier (original implementation by Lido) to verify the entire Validator Container in the CL.
 * It uses concatenated proofs against the beacon block root exposed in the EIP-4788 system contract.
 */
contract ValidatorContainerProofVerifier is AccessControl, IValidatorContainerProofVerifier {
  /**
   * @notice ValidatorContainerProofVerifier accepts concatenated Merkle proofs to verify existence of entire validator container on CL
   * Proof consists of:
   *  I:   Validator Container Root
   *  II:  Merkle proof of CL state - from Validator Container Root to State Root
   *  III: Merkle proof of Beacon block header - from State Root to Beacon block root
   *
   * In order to build proof you must collect all proofs from I, II, III and concatenate them into single array
   * We also concatenate GIndexes under the hood to properly traverse the superset tree up to the final root
   * Below is breakdown of each layer:
   */

  /*  Scheme of Validator Container Tree:

                            Validator Container Root                      **DEPTH = 0
                                        │
                        ┌───────────────┴───────────────┐
                        │                               │
                       node                            node               **DEPTH = 1
                        │                               │
                ┌───────┴───────┐               ┌───────┴───────┐
                │               │               │               │
               node            node            node            node       **DEPTH = 2
                │               │               │               │
          ┌─────┴─────┐   ┌─────┴─────┐   ┌─────┴─────┐   ┌─────┴─────┐
          │           │   │           │   │           │   │           │
       [pubkeyRoot]  [wc][EB]   [slashed][AEE]      [AE] [EE]       [WE]  **DEPTH = 3
       {................................................................}
                                        ↑
                                data to be proven
    */

  /**  GIndex of validator in state tree is calculated dynamically
     *   offsetting from GIndex of first validator by proving validator numerical index
     *
     *   Scheme of CL State Tree:
     *
                                CL State Tree                           **DEPTH = 0
                                        │
                        ┌───────────────┴───────────────┐
                        │                               │
             .......................................................
                │                               │
          ┌─────┴─────┐                   ┌─────┴─────┐
          │           │   ............... │           │
    [Validator 0]                        ....     [Validator to prove]  **DEPTH = N
            ↑                                               ↑
    GI_FIRST_VALIDATOR                   GI_FIRST_VALIDATOR + validator_index
    */

  /// @notice GIndex of first validator in CL state tree
  /// @dev This index is relative to a state like: `BeaconState.validators[0]`.
  GIndex public GI_FIRST_VALIDATOR;
  /// @notice GIndex of pending partial withdrawals root in CL state tree
  GIndex public GI_PENDING_PARTIAL_WITHDRAWALS_ROOT;

  /**
     *   GIndex of stateRoot in Beacon Block state is
     *   unlikely to change and same between mainnet/testnets
     *   Scheme of Beacon Block Tree:
     *
                                Beacon Block Root(from EIP-4788 Beacon Roots Contract)
                                        │
                        ┌───────────────┴──────────────────────────┐
                        │                                          │
                        node                                      proof[2]        **DEPTH = 1
                        │                                          │
                ┌───────┴───────┐                          ┌───────┴───────┐
                │               │                          │               │
  used to -> proof[1]          node                      node             node     **DEPTH = 2
  verify slot   │               │                          │               │
      ┌─────────┴─────┐   ┌─────┴───────────┐        ┌─────┴─────┐     ┌───┴──┐
      │               │   │                 │        │           │     │      │
    [slot]  [proposerInd] [parentRoot] [stateRoot]  [bodyRoot]  [0]   [0]    [0]   **DEPTH = 3
       ↑                   (proof[0])       ↑
    needed for GIndex                  what needs to be proven
     */
  uint8 private constant STATE_ROOT_DEPTH = 3;
  uint256 private constant STATE_ROOT_POSITION = 3;
  /// @notice GIndex of state root in Beacon block header
  GIndex public immutable GI_STATE_ROOT = pack((1 << STATE_ROOT_DEPTH) + STATE_ROOT_POSITION, STATE_ROOT_DEPTH);

  /// @notice location(from end) of parent node for (slot,proposerInd) in concatenated merkle proof
  uint256 private constant SLOT_PROPOSER_PARENT_PROOF_OFFSET = 2;

  /// @notice see `BEACON_ROOTS_ADDRESS` constant in the EIP-4788.
  address public constant BEACON_ROOTS = 0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02;

  // Sentinel value that a validator has no current exit scheduled
  uint64 public constant FAR_FUTURE_EXIT_EPOCH = type(uint64).max;

  // Validator must be active for this many epochs before it is eligible for withdrawals
  uint64 private constant SHARD_COMMITTEE_PERIOD = 256;
  uint64 private constant SLOTS_PER_EPOCH = 32;

  /// @notice Emitted when GI_FIRST_VALIDATOR is updated
  /// @param oldValue The previous GIndex value
  /// @param newValue The new GIndex value
  event GIFirstValidatorUpdated(GIndex oldValue, GIndex newValue);

  /// @notice Emitted when GI_PENDING_PARTIAL_WITHDRAWALS_ROOT is updated
  /// @param oldValue The previous GIndex value
  /// @param newValue The new GIndex value
  event GIPendingPartialWithdrawalsRootUpdated(GIndex oldValue, GIndex newValue);

  /**
   * @param _admin Address to be granted DEFAULT_ADMIN_ROLE
   * @param _gIFirstValidator packed(general index | depth in Merkle tree, see GIndex.sol) GIndex of first validator in CL state tree
   * @param _gIPendingPartialWithdrawalsRoot packed GIndex of pending partial withdrawals root in CL state tree
   */
  constructor(address _admin, GIndex _gIFirstValidator, GIndex _gIPendingPartialWithdrawalsRoot) {
    ErrorUtils.revertIfZeroAddress(_admin);
    ErrorUtils.revertIfZeroHash(GIndex.unwrap(_gIFirstValidator));
    ErrorUtils.revertIfZeroHash(GIndex.unwrap(_gIPendingPartialWithdrawalsRoot));
    _grantRole(DEFAULT_ADMIN_ROLE, _admin);
    GI_FIRST_VALIDATOR = _gIFirstValidator;
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT = _gIPendingPartialWithdrawalsRoot;
  }

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
  ) external view {
    // verifies user provided slot against user provided proof
    // proof verification is done in `SSZ.verifyProof` and is not affected by slot
    _verifySlot(_slot, _proposerIndex, _witness.proof);
    _validateActivationEpoch(_slot, _witness.activationEpoch);

    Validator memory validator = Validator({
      pubkey: _pubkey,
      withdrawalCredentials: _withdrawalCredentials,
      effectiveBalance: _witness.effectiveBalance,
      // No toleration for slashed validators
      slashed: false,
      activationEligibilityEpoch: _witness.activationEligibilityEpoch,
      activationEpoch: _witness.activationEpoch,
      // No toleration for validators pending exit
      exitEpoch: FAR_FUTURE_EXIT_EPOCH,
      // No toleration for validators pending full withdrawal
      withdrawableEpoch: FAR_FUTURE_EXIT_EPOCH
    });

    bytes32 validatorContainerRootLeaf = SSZ.hashTreeRoot(validator);

    // concatenated GIndex for
    // Validator Container Root -> Validator Index in state tree -> stateView Index in Beacon block Tree
    GIndex gIndex = concat(GI_STATE_ROOT, _getValidatorGI(_validatorIndex));

    SSZ.verifyProof({
      proof: _witness.proof,
      root: _getParentBlockRoot(_childBlockTimestamp),
      leaf: validatorContainerRootLeaf,
      gI: gIndex
    });
  }

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
  ) external view {
    _verifySlot(_slot, _proposerIndex, _witness.proof);
    bytes32 pendingPartialWithdrawalsRoot = SSZ.hashTreeRoot(_witness.pendingPartialWithdrawals);
    GIndex gIndex = concat(GI_STATE_ROOT, GI_PENDING_PARTIAL_WITHDRAWALS_ROOT);

    SSZ.verifyProof({
      proof: _witness.proof,
      root: _getParentBlockRoot(_childBlockTimestamp),
      leaf: pendingPartialWithdrawalsRoot,
      gI: gIndex
    });
  }

  /**
   * @notice Verifies that the provided slot and proposerIndex match the Merkle proof
   * @param _slot slot of the beacon block for which the proof is generated
   * @param _proposerIndex proposer index of the beacon block for which the proof is generated
   * @param _proof array of merkle proofs from Validator container root to Beacon block root
   * @dev Checks that the hash of (slot, proposerIndex) matches the parent node at proof[proof.length - 2]
   * This verifies that the slot and proposerIndex are part of the beacon block header in the proof
   * @dev Reverts with `InvalidSlot` if the slot/proposerIndex don't match the proof
   */
  function _verifySlot(uint64 _slot, uint64 _proposerIndex, bytes32[] calldata _proof) internal view {
    bytes32 parentSlotProposer = SSZ.sha256Pair(SSZ.toLittleEndian(_slot), SSZ.toLittleEndian(_proposerIndex));
    if (_proof[_proof.length - SLOT_PROPOSER_PARENT_PROOF_OFFSET] != parentSlotProposer) {
      revert InvalidSlot();
    }
  }

  /**
   * @notice Ensures the tracked validator has satisfied the post-activation waiting period.
   * @param _slot slot of the beacon block for which the proof is generated
   * @param _activationEpoch Activation epoch for the validator
   * @dev Reverts with `ValidatorNotActiveForLongEnough` when the validator has not remained active
   *      for at least `SHARD_COMMITTEE_PERIOD` epochs since `activationEpoch`.
   */
  function _validateActivationEpoch(uint64 _slot, uint64 _activationEpoch) internal pure {
    uint64 epoch = _slot / SLOTS_PER_EPOCH;
    if (epoch < _activationEpoch + SHARD_COMMITTEE_PERIOD) {
      revert ValidatorNotActiveForLongEnough();
    }
  }

  /**
   * @notice calculates general validator index in CL state tree by provided offset
   * @param _offset from first validator (Validator Index)
   * @return gIndex of container in CL state tree
   */
  function _getValidatorGI(uint256 _offset) internal view returns (GIndex) {
    return GI_FIRST_VALIDATOR.shr(_offset);
  }

  /**
   * @notice returns parent CL block root for given child block timestamp
   * @param _childBlockTimestamp timestamp of child block
   * @return parent block root
   * @dev reverts with `RootNotFound` if timestamp is not found in Beacon Block roots
   */
  function _getParentBlockRoot(uint64 _childBlockTimestamp) internal view returns (bytes32) {
    (bool success, bytes memory data) = BEACON_ROOTS.staticcall(abi.encode(_childBlockTimestamp));

    if (!success || data.length == 0) {
      revert RootNotFound();
    }

    return abi.decode(data, (bytes32));
  }

  /**
   * @notice Sets the GIndex of first validator in CL state tree
   * @param _gIFirstValidator packed GIndex of first validator in CL state tree
   * @dev Only callable by accounts with DEFAULT_ADMIN_ROLE
   */
  function setGIFirstValidator(GIndex _gIFirstValidator) external onlyRole(DEFAULT_ADMIN_ROLE) {
    ErrorUtils.revertIfZeroHash(GIndex.unwrap(_gIFirstValidator));
    GIndex oldValue = GI_FIRST_VALIDATOR;
    GI_FIRST_VALIDATOR = _gIFirstValidator;
    emit GIFirstValidatorUpdated(oldValue, _gIFirstValidator);
  }

  /**
   * @notice Sets the GIndex of pending partial withdrawals root in CL state tree
   * @param _gIPendingPartialWithdrawalsRoot packed GIndex of pending partial withdrawals root in CL state tree
   * @dev Only callable by accounts with DEFAULT_ADMIN_ROLE
   */
  function setGIPendingPartialWithdrawalsRoot(
    GIndex _gIPendingPartialWithdrawalsRoot
  ) external onlyRole(DEFAULT_ADMIN_ROLE) {
    ErrorUtils.revertIfZeroHash(GIndex.unwrap(_gIPendingPartialWithdrawalsRoot));
    GIndex oldValue = GI_PENDING_PARTIAL_WITHDRAWALS_ROOT;
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT = _gIPendingPartialWithdrawalsRoot;
    emit GIPendingPartialWithdrawalsRootUpdated(oldValue, _gIPendingPartialWithdrawalsRoot);
  }
}
