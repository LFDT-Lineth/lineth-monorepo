// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { IPauseManager } from "../../security/pausing/interfaces/IPauseManager.sol";
import { IPermissionsManager } from "../../security/access/interfaces/IPermissionsManager.sol";

/**
 * @title LinethRollup interface for current functions, structs, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ILinethRollupBase {
  /**
   * @notice Initialization data structure for the LinethRollup contract.
   * @param initialBlockHash The initial L2 block hash at initialization used for proof verification.
   * @param initialL2BlockNumber The initial block number at initialization.
   * @param genesisTimestamp The L2 genesis timestamp for first initialization.
   * @param defaultVerifier The default verifier for rollup proofs.
   * @param rateLimitPeriodInSeconds The period in which withdrawal amounts and fees will be accumulated.
   * @param rateLimitAmountInWei The limit allowed for withdrawing in the rate limit period.
   * @param roleAddresses The list of role address and roles to assign permissions to.
   * @param pauseTypeRoles The list of pause types to associate with roles.
   * @param unpauseTypeRoles The list of unpause types to associate with roles.
   * @param verifierKeys The initial set of allowed guest-program verifier keys.
   * @param defaultAdmin The account to be given DEFAULT_ADMIN_ROLE on initialization.
   * @param shnarfProvider The address of the shnarf providing contract. Default is address(this).
   * @param addressFilter The address of the address filter.
   */
  struct BaseInitializationData {
    bytes32 initialBlockHash;
    uint256 initialL2BlockNumber;
    uint256 genesisTimestamp;
    address defaultVerifier;
    uint256 rateLimitPeriodInSeconds;
    uint256 rateLimitAmountInWei;
    IPermissionsManager.RoleAddress[] roleAddresses;
    IPauseManager.PauseTypeRole[] pauseTypeRoles;
    IPauseManager.PauseTypeRole[] unpauseTypeRoles;
    bytes32[] verifierKeys;
    address defaultAdmin;
    address shnarfProvider;
    address addressFilter;
  }

  /**
   * @notice Data-availability stream position supplied during finalization.
   * @dev A stream position is the (dataRollingHash, offset) pair from the blob-spanning spec:
   *   the dataRollingHash is the 2-input accumulator after folding every chunk up to and including the
   *   one containing the position; the offset is the number of bytes consumed of that last-folded chunk.
   *   The previously-finalized position is supplied as calldata so the contract can open the stored
   *   position commitment (keccak256(dataRollingHash || offset)) rather than storing it in the clear.
   * @param prevDataRollingHash The previously-finalized end dataRollingHash (parent position accumulator).
   * @param prevOffset The previously-finalized end offset within its chunk (0 == fresh-start sentinel).
   */
  struct StreamPosition {
    bytes32 prevDataRollingHash;
    uint256 prevOffset;
  }

  /**
   * @notice Supporting data for finalization with proof.
   * @dev NB: the dynamic sized fields are placed last on purpose for efficient keccaking on public input.
   * @dev V6 replaces the shnarf linkage with the blob-spanning dataRollingHash stream-position model.
   * @param parentStateRootHash is the expected last state root hash finalized. Used only in the migration path.
   * @param parentBlockHash The expected L2 parent block hash at the start of this finalization. Execution-rooting continuity check.
   * @param endBlockNumber is the end block finalizing until.
   * @param lastFinalizedTimestamp is the expected last finalized block's timestamp.
   * @param finalTimestamp is the timestamp of the last block being finalized.
   * @param lastFinalizedL1RollingHash is the last stored L2 computed rolling hash used in finalization.
   * @param l1RollingHash is the calculated rolling hash on L2 that is expected to match L1 at l1RollingHashMessageNumber.
   * @param lastFinalizedL1RollingHashMessageNumber is the last stored L2 computed message number used in finalization.
   * @param l1RollingHashMessageNumber is the calculated message number on L2 that is expected to match the existing L1 rolling hash.
   * @param l2MerkleTreesDepth is the depth of all l2MerkleRoots.
   * @param lastFinalizedForcedTransactionNumber is the last proven forced transaction number processed on L2.
   * @param finalForcedTransactionNumber is the final forced transaction being finalized.
   * @param lastFinalizedForcedTransactionRollingHash is the last proven forced transaction rolling hash.
   * @param finalBlockHash The L2 final block hash that the current finalization ends on.
   * @param prevDataRollingHash The previously-finalized end dataRollingHash supplied to open the stored position commitment.
   * @param prevOffset The previously-finalized end offset within its chunk (0 == fresh-start sentinel).
   * @param parentDataRollingHash The parent dataRollingHash this finalization range starts from.
   * @param endDataRollingHash The end dataRollingHash this finalization range finishes at. Must have been anchored by a prior submission.
   * @param startOffset The starting stream offset of this finalization range.
   * @param endOffset The ending stream offset of this finalization range (bytes consumed of the last chunk).
   * @param l2MerkleRoots is an array of L2 message Merkle roots of depth l2MerkleTreesDepth between last finalized block and finalSubmissionData.finalBlockNumber.
   * @param filteredAddresses is an array of addresses that are filtered from forced transactions.
   * @param verifierKeys is an array of guest-program verifier keys used in this finalization batch.
   * @param l2MessagingBlocksOffsets indicates by offset from currentL2BlockNumber which L2 blocks contain MessageSent events.
   */
  struct FinalizationDataV6 {
    bytes32 parentStateRootHash;
    bytes32 parentBlockHash;
    uint256 endBlockNumber;
    uint256 lastFinalizedTimestamp;
    uint256 finalTimestamp;
    bytes32 lastFinalizedL1RollingHash;
    bytes32 l1RollingHash;
    uint256 lastFinalizedL1RollingHashMessageNumber;
    uint256 l1RollingHashMessageNumber;
    uint256 l2MerkleTreesDepth;
    uint256 lastFinalizedForcedTransactionNumber;
    uint256 finalForcedTransactionNumber;
    bytes32 lastFinalizedForcedTransactionRollingHash;
    bytes32 finalBlockHash;
    bytes32 prevDataRollingHash;
    uint256 prevOffset;
    bytes32 parentDataRollingHash;
    bytes32 endDataRollingHash;
    uint256 startOffset;
    uint256 endOffset;
    bytes32[] l2MerkleRoots;
    address[] filteredAddresses;
    bytes32[] verifierKeys;
    bytes l2MessagingBlocksOffsets;
  }

  /**
   * @notice Emitted when the LinethRollup contract version has changed.
   * @dev All bytes8 values are string based SemVer in the format M.m - e.g. "6.0".
   * @param previousVersion The previous version.
   * @param newVersion The new version.
   */
  event LineaRollupVersionChanged(bytes8 indexed previousVersion, bytes8 indexed newVersion);

  /**
   * @notice Emitted when a verifier is set for a particular proof type.
   * @param verifierAddress The indexed new verifier address being set.
   * @param proofType The indexed proof type/index that the verifier is mapped to.
   * @param verifierSetBy The index address who set the verifier at the mapping.
   * @param oldVerifierAddress Indicates the previous address mapped to the proof type.
   * @dev The verifier will be set by an account with the VERIFIER_SETTER_ROLE. Typically the Safe.
   * @dev The oldVerifierAddress can be the zero address.
   */
  event VerifierAddressChanged(
    address indexed verifierAddress,
    uint256 indexed proofType,
    address indexed verifierSetBy,
    address oldVerifierAddress
  );

  /**
   * @notice Emitted when guest-program verifier keys are added to the allowed set.
   * @param verifierKeys The verifier keys being set.
   */
  event VerifierKeysSet(bytes32[] verifierKeys);

  /**
   * @notice Emitted when guest-program verifier keys are removed from the allowed set.
   * @param verifierKeys The verifier keys being unset.
   */
  event VerifierKeysUnset(bytes32[] verifierKeys);

  /**
   * @notice Emitted when L2 blocks have been finalized on L1.
   * @param startBlockNumber The indexed L2 block number indicating which block the finalization the data starts from.
   * @param endBlockNumber The indexed L2 block number indicating which block the finalization the data ends on.
   * @param endDataRollingHash The indexed end dataRollingHash of the finalized DA stream position.
   * @param endOffset The end offset within the last-folded chunk of the finalized DA stream position.
   * @param parentBlockHash The parent L2 block hash that the current finalization starts from.
   *   Will be EMPTY_HASH on the first post-upgrade finalization (migration marker).
   * @param finalBlockHash The L2 block hash that the current finalization ends on.
   */
  event DataFinalizedV4(
    uint256 indexed startBlockNumber,
    uint256 indexed endBlockNumber,
    bytes32 indexed endDataRollingHash,
    uint256 endOffset,
    bytes32 parentBlockHash,
    bytes32 finalBlockHash
  );

  /**
   * @notice Emitted when L2 blocks have been finalized and the state is updated.
   * @dev Message rolling hash, forced transaction rolling hash, can be retrieved by using their numbered values.
   * @dev These fields can be used to reconstruct the values for `lastFinalizedState`.
   * @dev The `lastFinalizedState` fields are needed for liveness recovery as well as forced transactions.
   * @param blockNumber The indexed L2 block number indicating which block the finalization the data ends on.
   * @param timestamp The timestamp of the last block being finalized.
   * @param messageNumber The calculated message number on L2 that is expected to match the existing L1 rolling hash.
   * @param forcedTransactionNumber The final forced transaction finalized.
   */
  event FinalizedStateUpdated(
    uint256 indexed blockNumber,
    uint256 timestamp,
    uint256 messageNumber,
    uint256 forcedTransactionNumber
  );

  /**
   * @notice Emitted when the LinethRollupBase contract is initialized.
   * @param initialContractVersion The initial contract version.
   * @param initializationData The initialization data.
   * @param genesisShnarf The genesis shnarf.
   */
  event LineaRollupBaseInitialized(
    bytes8 indexed initialContractVersion,
    BaseInitializationData initializationData,
    bytes32 genesisShnarf
  );

  /**
   * @dev Thrown when finalizationData.l1RollingHash does not exist on L1 (Feedback loop).
   */
  error L1RollingHashDoesNotExistOnL1(uint256 messageNumber, bytes32 rollingHash);

  /**
   * @dev Thrown when finalization state does not match.
   */
  error FinalizationStateIncorrect(bytes32 expected, bytes32 value);

  /**
   * @dev Thrown when the final block hash equals the zero hash during finalization.
   */
  error FinalizationBlockHashIsZeroHash();

  /**
   * @dev Thrown when the starting block hash does not match the stored block hash on the new finalization path.
   */
  error StartingBlockHashDoesNotMatch();

  /**
   * @dev Thrown when final l2 block timestamp higher than current block.timestamp during finalization.
   */
  error FinalizationInTheFuture(uint256 l2BlockTimestamp, uint256 currentBlockTimestamp);

  /**
   * @dev Thrown when a rolling hash is provided without a corresponding message number.
   */
  error MissingMessageNumberForRollingHash(bytes32 rollingHash);

  /**
   * @dev Thrown when a message number is provided without a corresponding rolling hash.
   */
  error MissingRollingHashForMessageNumber(uint256 messageNumber);

  /**
   * @dev Thrown when a final dataRollingHash being finalized was not anchored by a prior submission.
   */
  error FinalDataRollingHashNotAnchored(bytes32 dataRollingHash);

  /**
   * @dev Thrown when the supplied previous stream position does not open the stored position commitment.
   */
  error PositionCommitmentMismatch(bytes32 expected, bytes32 value);

  /**
   * @dev Thrown when the parent dataRollingHash does not continue the previously-finalized position.
   */
  error DataRollingHashNotContinuous(bytes32 expected, bytes32 value);

  /**
   * @dev Thrown when the start offset neither continues the previously-finalized offset nor is a fresh start.
   */
  error StartOffsetNotContinuous(uint256 previousOffset, uint256 startOffset);

  /**
   * @dev Thrown when the shnarf supplied to the migration bridge does not match the live finalized value.
   */
  error BridgedShnarfMismatch(bytes32 expected, bytes32 value);

  /**
   * @dev Thrown when the rollup is missing a forced transaction in the finalization block range.
   */
  error FinalizationDataMissingForcedTransaction(uint256 nextForcedTransactionNumber);

  /**
   * @dev Thrown when an address is not filtered and expected to be.
   */
  error AddressIsNotFiltered(address addressNotFiltered);

  /**
   * @dev Thrown when the verifier keys array provided is empty.
   */
  error VerifierKeysEmpty();

  /**
   * @dev Thrown when a verifier key is not found in the allowed set.
   */
  error VerifierKeyNotFound(bytes32 verifierKey);

  /**
   * @dev Thrown when a verifier key is already set in the allowed set.
   */
  error VerifierKeyAlreadySet(bytes32 verifierKey);

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() external view returns (string memory contractVersion);

  /**
   * @notice Adds or updates the verifier contract address for a proof type.
   * @dev VERIFIER_SETTER_ROLE is required to execute.
   * @param _newVerifierAddress The address for the verifier contract.
   * @param _proofType The proof type being set/updated.
   */
  function setVerifierAddress(address _newVerifierAddress, uint256 _proofType) external;

  /**
   * @notice Unsets the verifier contract address for a proof type.
   * @dev VERIFIER_UNSETTER_ROLE is required to execute.
   * @param _proofType The proof type being set/updated.
   */
  function unsetVerifierAddress(uint256 _proofType) external;

  /**
   * @notice Adds verifier keys to the allowed set.
   * @dev SET_VERIFIER_KEY_ROLE is required to execute.
   * @param _verifierKeys The verifier keys to add.
   */
  function setVerifierKeys(bytes32[] calldata _verifierKeys) external;

  /**
   * @notice Removes verifier keys from the allowed set.
   * @dev UNSET_VERIFIER_KEY_ROLE is required to execute.
   * @param _verifierKeys The verifier keys to remove.
   */
  function unsetVerifierKeys(bytes32[] calldata _verifierKeys) external;

  /**
   * @notice Finalize compressed blocks with proof.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _aggregatedProof The aggregated proof.
   * @param _proofType The proof type.
   * @param _finalizationData The full finalization data.
   */
  function finalizeBlocks(
    bytes calldata _aggregatedProof,
    uint256 _proofType,
    FinalizationDataV6 calldata _finalizationData
  ) external;
}
