// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L1MessageService } from "../messaging/l1/L1MessageService.sol";
import { ZkEvmV2 } from "./ZkEvmV2.sol";
import { ILinethRollupBase } from "./interfaces/ILinethRollupBase.sol";
import { IProvideShnarf } from "./dataAvailability/interfaces/IProvideShnarf.sol";
import { PermissionsManager } from "../security/access/PermissionsManager.sol";
import { IPlonkVerifier } from "../verifiers/interfaces/IPlonkVerifier.sol";
import { FinalizedStateHashing } from "../libraries/FinalizedStateHashing.sol";
import { IAcceptForcedTransactions } from "./forcedTransactions/interfaces/IAcceptForcedTransactions.sol";
import { IGenericErrors } from "../interfaces/IGenericErrors.sol";
import { IAddressFilter } from "./forcedTransactions/interfaces/IAddressFilter.sol";

/**
 * @title Contract to manage cross-chain messaging on L1, L2 data submission, and rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LinethRollupBase is
  AccessControlUpgradeable,
  ZkEvmV2,
  L1MessageService,
  PermissionsManager,
  IAcceptForcedTransactions,
  ILinethRollupBase,
  IProvideShnarf
{
  /// @notice The role required to set/add proof verifiers by type.
  bytes32 public constant VERIFIER_SETTER_ROLE = keccak256("VERIFIER_SETTER_ROLE");

  /// @notice The role required to unset proof verifiers by type.
  bytes32 public constant VERIFIER_UNSETTER_ROLE = keccak256("VERIFIER_UNSETTER_ROLE");

  /// @notice The role required to set the address filter.
  bytes32 public constant SET_ADDRESS_FILTER_ROLE = keccak256("SET_ADDRESS_FILTER_ROLE");

  /// @notice The role required to send forced transactions.
  bytes32 public constant FORCED_TRANSACTION_SENDER_ROLE = keccak256("FORCED_TRANSACTION_SENDER_ROLE");

  /// @notice The role required to set the forced transaction fee.
  bytes32 public constant FORCED_TRANSACTION_FEE_SETTER_ROLE = keccak256("FORCED_TRANSACTION_FEE_SETTER_ROLE");

  /// @notice The role required to add guest-program verifier keys.
  bytes32 public constant SET_VERIFIER_KEY_ROLE = keccak256("SET_VERIFIER_KEY_ROLE");

  /// @notice The role required to remove guest-program verifier keys.
  bytes32 public constant UNSET_VERIFIER_KEY_ROLE = keccak256("UNSET_VERIFIER_KEY_ROLE");

  /// @notice The empty hash value.
  bytes32 internal constant EMPTY_HASH = 0x0;

  /// @notice This is the ABI version and not the reinitialize version.
  string private constant _CONTRACT_VERSION = "9.0";

  /// @dev DEPRECATED in favor of the single _blobShnarfExists mapping.
  mapping(bytes32 dataHash => bytes32 finalStateRootHash) private dataFinalStateRootHashes_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _blobShnarfExists mapping.
  mapping(bytes32 dataHash => bytes32 parentHash) private dataParents_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _blobShnarfExists mapping.
  mapping(bytes32 dataHash => bytes32 shnarfHash) private dataShnarfHashes_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _blobShnarfExists mapping.
  mapping(bytes32 dataHash => uint256 startingBlock) private dataStartingBlock_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _blobShnarfExists mapping.
  mapping(bytes32 dataHash => uint256 endingBlock) private dataEndingBlock_DEPRECATED;

  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  uint256 private currentL2StoredL1MessageNumber_DEPRECATED;
  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  bytes32 private currentL2StoredL1RollingHash_DEPRECATED;

  /**
   * @notice Commitment to the most recent finalized DA stream position.
   * @dev Holds keccak256(endDataRollingHash || encodeOffset(endOffset)) — the sealed position
   *   commitment from the blob-spanning spec. This occupies the same storage slot that previously
   *   held the plain finalized shnarf, so no storage-layout shift occurs. The position preimage
   *   (dataRollingHash, offset) is supplied as calldata by the next finalization.
   */
  bytes32 public currentFinalizedShnarf;

  /**
   * @dev NB: THIS IS THE ONLY MAPPING BEING USED FOR DATA SUBMISSION TRACKING.
   * @dev NB: Keys are anchored dataRollingHash values (previously shnarfs). Only the final
   *   dataRollingHash of each submission is anchored; intermediate chunk folds are not persisted.
   *   Membership-only — execution continuity no longer travels with the DA accumulator.
   */
  mapping(bytes32 dataRollingHash => uint256 exists) internal _blobShnarfExists;

  /**
   * @notice Hash of the L2 computed message number, its rolling hash,
   * forced transaction number and its rolling hash,
   * and the L2 block timestamp.
   */
  bytes32 public currentFinalizedState;

  /// @notice The address of the liveness recovery operator.
  /// @dev This address is granted the OPERATOR_ROLE after six months of finalization inactivity by the current operators.
  address public livenessRecoveryOperator;

  /// @notice The address of the shnarf provider.
  /// @dev Default is address(this).
  IProvideShnarf public shnarfProvider;

  /// @dev The unique forced transaction number.
  uint256 public nextForcedTransactionNumber;

  /// @dev The expected L2 block numbers for forced transactions.
  mapping(uint256 forcedTransactionNumber => uint256 l2BlockNumber) public forcedTransactionL2BlockNumbers;

  /// @dev The rolling hash for a forced transaction.
  mapping(uint256 forcedTransactionNumber => bytes32 rollingHash) public forcedTransactionRollingHashes;

  /// @dev The forced transaction fee in wei.
  uint256 public forcedTransactionFeeInWei;

  /// @notice The address of the address filter.
  IAddressFilter public addressFilter;

  /// @notice Allowed guest-program verifier keys, managed by SET_VERIFIER_KEY_ROLE / UNSET_VERIFIER_KEY_ROLE.
  mapping(bytes32 verifierKey => bool exists) public verifierKeys;

  /// @notice The L2 block hash stored per block number. Populated on finalization and at initialization.
  mapping(uint256 blockNumber => bytes32 blockHash) public blockHashes;

  /// @dev Keep 48 free storage slots for inheriting contracts (reduced from 50 to account for two new mappings above).
  uint256[48] private __gap_LineaRollup;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LinethRollup and underlying service dependencies - used for new networks only.
   * @param _initializationData The initial data used for contract initialization.
   * @param _genesisPositionCommitment The initial sealed position commitment for the genesis DA stream position.
   */
  function __LinethRollup_init(
    BaseInitializationData calldata _initializationData,
    bytes32 _genesisPositionCommitment
  ) internal virtual onlyInitializing {
    if (_initializationData.defaultVerifier == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    if (_initializationData.addressFilter == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    __PauseManager_init(_initializationData.pauseTypeRoles, _initializationData.unpauseTypeRoles);

    __MessageService_init(_initializationData.rateLimitPeriodInSeconds, _initializationData.rateLimitAmountInWei);

    if (_initializationData.defaultAdmin == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    /**
     * @dev DEFAULT_ADMIN_ROLE is set for the security council explicitly,
     * as the permissions init purposefully does not allow DEFAULT_ADMIN_ROLE to be set.
     */
    _grantRole(DEFAULT_ADMIN_ROLE, _initializationData.defaultAdmin);

    __Permissions_init(_initializationData.roleAddresses);

    verifiers[0] = _initializationData.defaultVerifier;

    require(_initializationData.initialBlockHash != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());
    currentL2BlockNumber = _initializationData.initialL2BlockNumber;
    blockHashes[_initializationData.initialL2BlockNumber] = _initializationData.initialBlockHash;

    currentFinalizedShnarf = _genesisPositionCommitment;
    currentFinalizedState = FinalizedStateHashing._computeLastFinalizedState(
      0,
      EMPTY_HASH,
      0,
      EMPTY_HASH,
      _initializationData.genesisTimestamp
    );

    nextForcedTransactionNumber = 1;

    address shnarfProviderAddress = _initializationData.shnarfProvider;

    if (shnarfProviderAddress == address(0)) {
      shnarfProviderAddress = address(this);
    }

    shnarfProvider = IProvideShnarf(shnarfProviderAddress);

    addressFilter = IAddressFilter(_initializationData.addressFilter);

    // Seed the initial allowed verifier keys.
    for (uint256 i; i < _initializationData.verifierKeys.length; i++) {
      require(_initializationData.verifierKeys[i] != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());
      verifierKeys[_initializationData.verifierKeys[i]] = true;
    }
    if (_initializationData.verifierKeys.length > 0) {
      emit VerifierKeysSet(_initializationData.verifierKeys);
    }

    emit LineaRollupBaseInitialized(bytes8(bytes(CONTRACT_VERSION())), _initializationData, _genesisPositionCommitment);
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() public view virtual returns (string memory contractVersion) {
    contractVersion = _CONTRACT_VERSION;
  }

  /**
   * @notice Provides state fields for forced transactions.
   * @return finalizedState The last finalized state hash.
   * @return previousForcedTransactionRollingHash The previous forced transaction rolling hash.
   * @return previousForcedTransactionBlockDeadline The previous forced transaction block deadline.
   * @return currentFinalizedL2BlockNumber The current finalized L2 block number.
   * @return forcedTransactionFeeAmount The forced transaction fee.
   */
  function getRequiredForcedTransactionFields()
    external
    view
    returns (
      bytes32 finalizedState,
      bytes32 previousForcedTransactionRollingHash,
      uint256 previousForcedTransactionBlockDeadline,
      uint256 currentFinalizedL2BlockNumber,
      uint256 forcedTransactionFeeAmount
    )
  {
    uint256 previousForcedTransactionNumber = nextForcedTransactionNumber - 1;
    unchecked {
      finalizedState = currentFinalizedState;
      previousForcedTransactionRollingHash = forcedTransactionRollingHashes[previousForcedTransactionNumber];
      previousForcedTransactionBlockDeadline = forcedTransactionL2BlockNumbers[previousForcedTransactionNumber];
      currentFinalizedL2BlockNumber = currentL2BlockNumber;
      forcedTransactionFeeAmount = forcedTransactionFeeInWei;
    }
  }

  /**
   * @notice Sets the forced transaction fee.
   * @dev FORCED_TRANSACTION_FEE_SETTER_ROLE is required to set the forced transaction fee.
   * @param _forcedTransactionFeeInWei The forced transaction fee in wei.
   */
  function setForcedTransactionFee(
    uint256 _forcedTransactionFeeInWei
  ) external onlyRole(FORCED_TRANSACTION_FEE_SETTER_ROLE) {
    require(_forcedTransactionFeeInWei > 0, IGenericErrors.ZeroValueNotAllowed());
    forcedTransactionFeeInWei = _forcedTransactionFeeInWei;
    emit ForcedTransactionFeeSet(_forcedTransactionFeeInWei);
  }

  /**
   * @notice Stores forced transaction details required for proving feedback loop.
   * @dev FORCED_TRANSACTION_SENDER_ROLE is required to store a forced transaction.
   * @dev The forced transaction number is incremented for the next transaction post storage.
   * @param _forcedTransactionRollingHash The rolling hash for all the forced transaction fields.
   * @param _from The recovered signer's from address.
   * @param _blockNumberDeadline The maximum expected L2 block number processing will occur by.
   * @param _rlpEncodedSignedTransaction The RLP encoded type 02 transaction payload including signature.
   */
  function storeForcedTransaction(
    bytes32 _forcedTransactionRollingHash,
    address _from,
    uint256 _blockNumberDeadline,
    bytes calldata _rlpEncodedSignedTransaction
  ) external payable virtual onlyRole(FORCED_TRANSACTION_SENDER_ROLE) {
    unchecked {
      require(_rlpEncodedSignedTransaction.length > 0, IGenericErrors.ZeroLengthNotAllowed());
      require(_blockNumberDeadline > 0, IGenericErrors.ZeroValueNotAllowed());
      require(_from != address(0), IGenericErrors.ZeroAddressNotAllowed());
      require(_forcedTransactionRollingHash != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());

      uint256 forcedTransactionNumber = nextForcedTransactionNumber++;

      require(
        forcedTransactionL2BlockNumbers[forcedTransactionNumber - 1] < _blockNumberDeadline,
        ForcedTransactionExistsForBlockOrIsTooLow(_blockNumberDeadline)
      );

      forcedTransactionRollingHashes[forcedTransactionNumber] = _forcedTransactionRollingHash;
      forcedTransactionL2BlockNumbers[forcedTransactionNumber] = _blockNumberDeadline;

      emit ForcedTransactionAdded(
        forcedTransactionNumber,
        _from,
        _blockNumberDeadline,
        _forcedTransactionRollingHash,
        _rlpEncodedSignedTransaction
      );
    }
  }

  /**
   * @notice Adds or updates the verifier contract address for a proof type.
   * @dev VERIFIER_SETTER_ROLE is required to execute.
   * @param _newVerifierAddress The address for the verifier contract.
   * @param _proofType The proof type being set/updated.
   */
  function setVerifierAddress(address _newVerifierAddress, uint256 _proofType) external onlyRole(VERIFIER_SETTER_ROLE) {
    if (_newVerifierAddress == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    emit VerifierAddressChanged(_newVerifierAddress, _proofType, msg.sender, verifiers[_proofType]);

    verifiers[_proofType] = _newVerifierAddress;
  }

  /**
   * @notice Unset the verifier contract address for a proof type.
   * @dev VERIFIER_UNSETTER_ROLE is required to execute.
   * @param _proofType The proof type being set/updated.
   */
  function unsetVerifierAddress(uint256 _proofType) external onlyRole(VERIFIER_UNSETTER_ROLE) {
    emit VerifierAddressChanged(address(0), _proofType, msg.sender, verifiers[_proofType]);

    delete verifiers[_proofType];
  }

  /**
   * @notice Adds guest-program verifier keys to the allowed set.
   * @dev SET_VERIFIER_KEY_ROLE is required to execute.
   * @param _verifierKeys The verifier keys to add.
   */
  function setVerifierKeys(bytes32[] calldata _verifierKeys) external onlyRole(SET_VERIFIER_KEY_ROLE) {
    require(_verifierKeys.length > 0, VerifierKeysEmpty());
    for (uint256 i; i < _verifierKeys.length; i++) {
      require(_verifierKeys[i] != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());
      require(!verifierKeys[_verifierKeys[i]], VerifierKeyAlreadySet(_verifierKeys[i]));
      verifierKeys[_verifierKeys[i]] = true;
    }
    emit VerifierKeysSet(_verifierKeys);
  }

  /**
   * @notice Removes guest-program verifier keys from the allowed set.
   * @dev UNSET_VERIFIER_KEY_ROLE is required to execute.
   * @param _verifierKeys The verifier keys to remove.
   */
  function unsetVerifierKeys(bytes32[] calldata _verifierKeys) external onlyRole(UNSET_VERIFIER_KEY_ROLE) {
    require(_verifierKeys.length > 0, VerifierKeysEmpty());
    for (uint256 i; i < _verifierKeys.length; i++) {
      require(verifierKeys[_verifierKeys[i]], VerifierKeyNotFound(_verifierKeys[i]));
      delete verifierKeys[_verifierKeys[i]];
    }
    emit VerifierKeysUnset(_verifierKeys);
  }

  /**
   * @notice Sets the address filter.
   * @dev SET_ADDRESS_FILTER_ROLE is required to execute.
   * @param _addressFilter The address filter value.
   */
  function setAddressFilter(address _addressFilter) external onlyRole(SET_ADDRESS_FILTER_ROLE) {
    require(_addressFilter != address(0), IGenericErrors.ZeroAddressNotAllowed());
    address oldAddressFilter = address(addressFilter);

    if (_addressFilter != oldAddressFilter) {
      addressFilter = IAddressFilter(_addressFilter);
      emit AddressFilterChanged(oldAddressFilter, _addressFilter);
    }
  }

  /**
   * @notice Internal function to compute the 2-input dataRollingHash fold.
   * @dev keccak256(parentDataRollingHash || chunkHash) — the pure DA accumulator from the
   *   blob-spanning spec. Using assembly this way is cheaper gas wise.
   * @param _parentDataRollingHash The dataRollingHash of the parent stream position.
   * @param _chunkHash The chunk hash: blobhash(i) (EIP-4844 versioned hash) for blobs,
   *   keccak256(compressedData) for calldata.
   * @return dataRollingHash The computed dataRollingHash.
   */
  function _computeDataRollingHash(
    bytes32 _parentDataRollingHash,
    bytes32 _chunkHash
  ) internal pure returns (bytes32 dataRollingHash) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _parentDataRollingHash)
      mstore(add(mPtr, 0x20), _chunkHash)
      dataRollingHash := keccak256(mPtr, 0x40)
    }
  }

  /**
   * @notice Internal function to compute the sealed position commitment for a stream position.
   * @dev keccak256(dataRollingHash || encodeOffset(offset)), where the offset is abi-encoded as a
   *   uint256 (32-byte big-endian). Stored in the `currentFinalizedShnarf` slot at finalization and
   *   opened by the next finalization supplying the (dataRollingHash, offset) preimage as calldata.
   * @param _dataRollingHash The dataRollingHash of the stream position.
   * @param _offset The byte offset within the last-folded chunk.
   * @return positionCommitment The computed position commitment.
   */
  function _computePositionCommitment(
    bytes32 _dataRollingHash,
    uint256 _offset
  ) internal pure returns (bytes32 positionCommitment) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _dataRollingHash)
      mstore(add(mPtr, 0x20), _offset)
      positionCommitment := keccak256(mPtr, 0x40)
    }
  }

  /**
   * @notice Computes the legacy 3-input block-hash-centric shnarf.
   * @dev Retained solely for the one-way migration bridge (converting the last finalized 3-arg
   *   shnarf into the initial dataRollingHash position commitment). New finalizations never call this.
   * @param _parentShnarf The shnarf of the parent data item.
   * @param _finalBlockHash The L2 final block hash for this data item.
   * @param _dataHash The data hash: blobhash(i) for EIP-4844 blobs, keccak256(compressedData) for calldata.
   * @return shnarf The computed shnarf.
   */
  function _computeShnarf(
    bytes32 _parentShnarf,
    bytes32 _finalBlockHash,
    bytes32 _dataHash
  ) internal pure returns (bytes32 shnarf) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _parentShnarf)
      mstore(add(mPtr, 0x20), _finalBlockHash)
      mstore(add(mPtr, 0x40), _dataHash)
      shnarf := keccak256(mPtr, 0x60)
    }
  }

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
  ) external virtual whenTypeAndGeneralNotPaused(PauseType.FINALIZATION) onlyRole(OPERATOR_ROLE) {
    if (_aggregatedProof.length == 0) {
      revert ProofIsEmpty();
    }

    uint256 lastFinalizedBlockNumber = currentL2BlockNumber;

    address verifier = verifiers[_proofType];

    if (verifier == address(0)) {
      revert InvalidProofType();
    }

    bytes32 finalForcedTransactionRollingHash = forcedTransactionRollingHashes[
      _finalizationData.finalForcedTransactionNumber
    ];

    if (_finalizationData.finalForcedTransactionNumber > 0 && finalForcedTransactionRollingHash == EMPTY_HASH) {
      revert MissingRollingHashForForcedTransactionNumber(_finalizationData.finalForcedTransactionNumber);
    }

    _validateVerifierKeys(_finalizationData.verifierKeys);

    /// @dev currentFinalizedShnarf (the position commitment) is updated in _finalizeBlocks and the
    ///   previous value MUST be captured beforehand to open the position commitment.
    bytes32 lastFinalizedPositionCommitment = currentFinalizedShnarf;

    _verifyProof(
      _computePublicInput(
        _finalizationData,
        lastFinalizedPositionCommitment,
        _finalizeBlocks(
          _finalizationData,
          lastFinalizedBlockNumber,
          finalForcedTransactionRollingHash,
          lastFinalizedPositionCommitment
        ),
        finalForcedTransactionRollingHash,
        IPlonkVerifier(verifier).getChainConfiguration()
      ),
      verifier,
      _aggregatedProof
    );
  }

  /**
   * @notice Internal function to finalize compressed blocks.
   * @dev If blockHashes[lastFinalizedBlock] is EMPTY_HASH, validates the legacy parent state root.
   *   DA linkage uses the blob-spanning position-commitment model: the stored position commitment is
   *   opened by the calldata-supplied previous stream position, DA continuity and anchoring are
   *   asserted, and execution rooting is checked explicitly against the block-hash chain.
   * @param _finalizationData The full finalization data.
   * @param _lastFinalizedBlock The last finalized block number.
   * @param _finalForcedTransactionRollingHash The rolling hash for the final forced transaction.
   * @param _lastFinalizedPositionCommitment The previously-stored position commitment to open.
   * @return positionCommitment The sealed position commitment for the finalized end stream position.
   */
  function _finalizeBlocks(
    FinalizationDataV6 calldata _finalizationData,
    uint256 _lastFinalizedBlock,
    bytes32 _finalForcedTransactionRollingHash,
    bytes32 _lastFinalizedPositionCommitment
  ) internal returns (bytes32 positionCommitment) {
    _validateL2ComputedRollingHash(_finalizationData.l1RollingHashMessageNumber, _finalizationData.l1RollingHash);

    _validateFilteredAddresses(_finalizationData.filteredAddresses);

    bytes32 lastFinalizedState = currentFinalizedState;
    if (
      FinalizedStateHashing._computeLastFinalizedState(
        _finalizationData.lastFinalizedL1RollingHashMessageNumber,
        _finalizationData.lastFinalizedL1RollingHash,
        _finalizationData.lastFinalizedForcedTransactionNumber,
        _finalizationData.lastFinalizedForcedTransactionRollingHash,
        _finalizationData.lastFinalizedTimestamp
      ) != lastFinalizedState
    ) {
      revert FinalizationStateIncorrect(
        lastFinalizedState,
        FinalizedStateHashing._computeLastFinalizedState(
          _finalizationData.lastFinalizedL1RollingHashMessageNumber,
          _finalizationData.lastFinalizedL1RollingHash,
          _finalizationData.lastFinalizedForcedTransactionNumber,
          _finalizationData.lastFinalizedForcedTransactionRollingHash,
          _finalizationData.lastFinalizedTimestamp
        )
      );
    }

    require(
      _finalizationData.finalTimestamp < block.timestamp,
      FinalizationInTheFuture(_finalizationData.finalTimestamp, block.timestamp)
    );

    /// @dev Check the next forced transaction is outside the scope of our finalization for censorship resistance checking.
    unchecked {
      uint256 nextFinalizationStartingForcedTxNumber = forcedTransactionL2BlockNumbers[
        _finalizationData.finalForcedTransactionNumber + 1
      ];

      require(
        nextFinalizationStartingForcedTxNumber == 0 ||
          nextFinalizationStartingForcedTxNumber > _finalizationData.endBlockNumber,
        FinalizationDataMissingForcedTransaction(_finalizationData.finalForcedTransactionNumber + 1)
      );
    }

    // Look up parent block hash BEFORE any state update.
    // EMPTY_HASH signals migration path: parent was committed under the old state-root-hash model.
    bytes32 parentBlockHash = blockHashes[_lastFinalizedBlock];

    if (parentBlockHash == EMPTY_HASH) {
      // MIGRATION PATH: parent was committed under the old state-root-hash model.
      require(_finalizationData.parentBlockHash == EMPTY_HASH, StartingBlockHashDoesNotMatch());
      bytes32 parentStateRootHash = stateRootHashes[_lastFinalizedBlock];
      require(
        parentStateRootHash != EMPTY_HASH && parentStateRootHash == _finalizationData.parentStateRootHash,
        StartingRootHashDoesNotMatch()
      );
    } else {
      // NEW PATH: parent committed under block-hash model.
      // Execution-rooting continuity check: the caller declares the parent block hash they build from;
      // the on-chain blockHashes mapping is authoritative.
      require(parentBlockHash == _finalizationData.parentBlockHash, StartingBlockHashDoesNotMatch());
    }

    require(_finalizationData.finalBlockHash != EMPTY_HASH, FinalizationBlockHashIsZeroHash());

    // Open the stored position commitment with the calldata-supplied previous stream position.
    bytes32 openedCommitment = _computePositionCommitment(
      _finalizationData.prevDataRollingHash,
      _finalizationData.prevOffset
    );
    require(
      openedCommitment == _lastFinalizedPositionCommitment,
      PositionCommitmentMismatch(_lastFinalizedPositionCommitment, openedCommitment)
    );

    // DA continuity: the range must start from the previously-finalized dataRollingHash.
    require(
      _finalizationData.parentDataRollingHash == _finalizationData.prevDataRollingHash,
      DataRollingHashNotContinuous(_finalizationData.prevDataRollingHash, _finalizationData.parentDataRollingHash)
    );

    // Position continuity: continue the previous offset, or fresh-start at a chunk boundary (offset 0).
    require(
      _finalizationData.startOffset == _finalizationData.prevOffset || _finalizationData.startOffset == 0,
      StartOffsetNotContinuous(_finalizationData.prevOffset, _finalizationData.startOffset)
    );

    // DA anchoring: the end dataRollingHash must have been anchored by a prior submission.
    require(
      shnarfProvider.blobShnarfExists(_finalizationData.endDataRollingHash) != 0,
      FinalDataRollingHashNotAnchored(_finalizationData.endDataRollingHash)
    );

    _addL2MerkleRoots(_finalizationData.l2MerkleRoots, _finalizationData.l2MerkleTreesDepth);
    _anchorL2MessagingBlocks(_finalizationData.l2MessagingBlocksOffsets, _lastFinalizedBlock);

    // Anchor the new block hash — moves the next finalization round onto the new path.
    blockHashes[_finalizationData.endBlockNumber] = _finalizationData.finalBlockHash;

    currentL2BlockNumber = _finalizationData.endBlockNumber;
    positionCommitment = _computePositionCommitment(_finalizationData.endDataRollingHash, _finalizationData.endOffset);
    currentFinalizedShnarf = positionCommitment;
    currentFinalizedState = FinalizedStateHashing._computeLastFinalizedState(
      _finalizationData.l1RollingHashMessageNumber,
      _finalizationData.l1RollingHash,
      _finalizationData.finalForcedTransactionNumber,
      _finalForcedTransactionRollingHash,
      _finalizationData.finalTimestamp
    );

    emit FinalizedStateUpdated(
      _finalizationData.endBlockNumber,
      _finalizationData.finalTimestamp,
      _finalizationData.l1RollingHashMessageNumber,
      _finalizationData.finalForcedTransactionNumber
    );

    unchecked {
      emit DataFinalizedV4(
        ++_lastFinalizedBlock,
        _finalizationData.endBlockNumber,
        _finalizationData.endDataRollingHash,
        _finalizationData.endOffset,
        parentBlockHash,
        _finalizationData.finalBlockHash
      );
    }
  }

  /**
   * @notice Internal function to validate filtered addresses.
   * @param _filteredAddresses The filtered addresses.
   */
  function _validateFilteredAddresses(address[] calldata _filteredAddresses) internal view {
    if (_filteredAddresses.length > 0) {
      IAddressFilter addressFilterCached = addressFilter;

      for (uint256 i; i < _filteredAddresses.length; i++) {
        require(
          addressFilterCached.addressIsFiltered(_filteredAddresses[i]),
          AddressIsNotFiltered(_filteredAddresses[i])
        );
      }
    }
  }

  /**
   * @notice Internal function to validate l1 rolling hash.
   * @param _rollingHashMessageNumber Message number associated with the rolling hash as computed on L2.
   * @param _rollingHash L1 rolling hash as computed on L2.
   */
  function _validateL2ComputedRollingHash(uint256 _rollingHashMessageNumber, bytes32 _rollingHash) internal view {
    if (_rollingHashMessageNumber == 0) {
      if (_rollingHash != EMPTY_HASH) {
        revert MissingMessageNumberForRollingHash(_rollingHash);
      }
    } else {
      if (_rollingHash == EMPTY_HASH) {
        revert MissingRollingHashForMessageNumber(_rollingHashMessageNumber);
      }
      if (rollingHashes[_rollingHashMessageNumber] != _rollingHash) {
        revert L1RollingHashDoesNotExistOnL1(_rollingHashMessageNumber, _rollingHash);
      }
    }
  }

  /**
   * @notice Internal function to validate that all verifier keys used in a finalization are in the allowed set.
   * @dev Applies to execution and compression proofs. The aggregation verifier manages its own key constraints.
   * @param _verifierKeysUsed The verifier keys used in the finalization batch.
   */
  function _validateVerifierKeys(bytes32[] calldata _verifierKeysUsed) internal view {
    for (uint256 i; i < _verifierKeysUsed.length; i++) {
      require(verifierKeys[_verifierKeysUsed[i]], VerifierKeyNotFound(_verifierKeysUsed[i]));
    }
  }

  /**
   * @notice Compute the public input.
   * @dev Binds the full 20-field blob-spanning public-input surface (rollup_spec §2.4), with the
   *   sealed position commitment standing in for the old shnarf pair. The dataRollingHash stream
   *   positions, the execution block-hash pair, and the L1/L2 rolling-hash, FTX, and Merkle fields
   *   are all committed so the proof is bound to exactly the state transition the contract checked.
   * @dev Computing the public input as:
   * keccak256(
   *  abi.encode(
   *     _lastFinalizedPositionCommitment,          // parent position commitment (prevDataRollingHash || prevOffset)
   *     _positionCommitment,                       // end position commitment (endDataRollingHash || endOffset)
   *     _finalizationData.parentDataRollingHash,
   *     _finalizationData.endDataRollingHash,
   *     _finalizationData.startOffset,
   *     _finalizationData.endOffset,
   *     _finalizationData.parentBlockHash,
   *     _finalizationData.finalBlockHash,
   *     _finalizationData.finalTimestamp,
   *     _finalizationData.endBlockNumber,
   *     _finalizationData.lastFinalizedL1RollingHash,
   *     _finalizationData.l1RollingHash,
   *     _finalizationData.lastFinalizedL1RollingHashMessageNumber,
   *     _finalizationData.l1RollingHashMessageNumber,
   *     _finalizationData.lastFinalizedForcedTransactionRollingHash,
   *     _finalForcedTransactionRollingHash,
   *     _finalizationData.lastFinalizedForcedTransactionNumber,
   *     _finalizationData.finalForcedTransactionNumber,
   *     _finalizationData.l2MerkleTreesDepth,
   *     keccak256(abi.encodePacked(_finalizationData.l2MerkleRoots)),
   *     _verifierChainConfiguration,
   *     keccak256(abi.encodePacked(_finalizationData.filteredAddresses)),
   *     keccak256(abi.encodePacked(_finalizationData.verifierKeys))
   *   )
   * )
   * @param _finalizationData The full finalization data.
   * @param _lastFinalizedPositionCommitment The previously-stored position commitment being opened.
   * @param _positionCommitment The sealed position commitment for the finalized end stream position.
   * @param _finalForcedTransactionRollingHash The final processed forced transactions's rolling hash.
   * @param _verifierChainConfiguration The verifier chain configuration.
   * @return publicInput The computed public input.
   */
  function _computePublicInput(
    FinalizationDataV6 calldata _finalizationData,
    bytes32 _lastFinalizedPositionCommitment,
    bytes32 _positionCommitment,
    bytes32 _finalForcedTransactionRollingHash,
    bytes32 _verifierChainConfiguration
  ) private pure returns (uint256 publicInput) {
    // For a `calldata` struct reference, the base points at the struct's leading offset word, so
    // struct field N sits at calldata offset (N+1)*0x20 from the base. FinalizationDataV6 has 19
    // fixed fields (field indices 0..18) at base-relative offsets 0x20..0x260; the four dynamic
    // offset pointers (l2MerkleRoots, filteredAddresses, verifierKeys, l2MessagingBlocksOffsets)
    // follow at base-relative 0x280, 0x2a0, 0x2c0, 0x2e0.
    assembly {
      let mPtr := mload(0x40)
      let fd := _finalizationData
      mstore(mPtr, _lastFinalizedPositionCommitment) // 0
      mstore(add(mPtr, 0x20), _positionCommitment) // 1
      calldatacopy(add(mPtr, 0x40), add(fd, 0x220), 0x20) // 2: parentDataRollingHash (field 16)
      calldatacopy(add(mPtr, 0x60), add(fd, 0x240), 0x20) // 3: endDataRollingHash (field 17)
      calldatacopy(add(mPtr, 0x80), add(fd, 0x260), 0x20) // 4: startOffset (field 18)
      calldatacopy(add(mPtr, 0xa0), add(fd, 0x280), 0x20) // 5: endOffset (field 19)
      calldatacopy(add(mPtr, 0xc0), add(fd, 0x40), 0x20) // 6: parentBlockHash (field 1)
      calldatacopy(add(mPtr, 0xe0), add(fd, 0x1c0), 0x20) // 7: finalBlockHash (field 13)
      calldatacopy(add(mPtr, 0x100), add(fd, 0xa0), 0x20) // 8: finalTimestamp (field 4)
      calldatacopy(add(mPtr, 0x120), add(fd, 0x60), 0x20) // 9: endBlockNumber (field 2)
      calldatacopy(add(mPtr, 0x140), add(fd, 0xc0), 0x20) // 10: lastFinalizedL1RollingHash (field 5)
      calldatacopy(add(mPtr, 0x160), add(fd, 0xe0), 0x20) // 11: l1RollingHash (field 6)
      calldatacopy(add(mPtr, 0x180), add(fd, 0x100), 0x20) // 12: lastFinalizedL1RollingHashMessageNumber (field 7)
      calldatacopy(add(mPtr, 0x1a0), add(fd, 0x120), 0x20) // 13: l1RollingHashMessageNumber (field 8)
      calldatacopy(add(mPtr, 0x1c0), add(fd, 0x1a0), 0x20) // 14: lastFinalizedForcedTransactionRollingHash (field 12)
      mstore(add(mPtr, 0x1e0), _finalForcedTransactionRollingHash) // 15
      calldatacopy(add(mPtr, 0x200), add(fd, 0x160), 0x20) // 16: lastFinalizedForcedTransactionNumber (field 10)
      calldatacopy(add(mPtr, 0x220), add(fd, 0x180), 0x20) // 17: finalForcedTransactionNumber (field 11)
      calldatacopy(add(mPtr, 0x240), add(fd, 0x140), 0x20) // 18: l2MerkleTreesDepth (field 9)

      // 19: keccak256(abi.encodePacked(l2MerkleRoots)) — offset pointer at fd+0x280.
      let rootsLenLoc := add(fd, calldataload(add(fd, 0x280)))
      let rootsLen := calldataload(rootsLenLoc)
      let rootsPtr := add(mPtr, 0x2e0)
      calldatacopy(rootsPtr, add(rootsLenLoc, 0x20), mul(rootsLen, 0x20))
      mstore(add(mPtr, 0x260), keccak256(rootsPtr, mul(rootsLen, 0x20)))

      mstore(add(mPtr, 0x280), _verifierChainConfiguration) // 20

      // 21: keccak256(abi.encodePacked(filteredAddresses)) — offset pointer at fd+0x2a0.
      let filtLenLoc := add(fd, calldataload(add(fd, 0x2a0)))
      let filtLen := calldataload(filtLenLoc)
      let filtPtr := add(mPtr, 0x2e0)
      calldatacopy(filtPtr, add(filtLenLoc, 0x20), mul(filtLen, 0x20))
      mstore(add(mPtr, 0x2a0), keccak256(filtPtr, mul(filtLen, 0x20)))

      // 22: keccak256(abi.encodePacked(verifierKeys)) — offset pointer at fd+0x2c0.
      let vkLenLoc := add(fd, calldataload(add(fd, 0x2c0)))
      let vkLen := calldataload(vkLenLoc)
      let vkPtr := add(mPtr, 0x2e0)
      calldatacopy(vkPtr, add(vkLenLoc, 0x20), mul(vkLen, 0x20))
      mstore(add(mPtr, 0x2c0), keccak256(vkPtr, mul(vkLen, 0x20)))

      publicInput := mod(keccak256(mPtr, 0x2e0), MODULO_R)
    }
  }

  /**
   * @notice Verifies the proof with locally computed public inputs.
   * @dev If the verifier based on proof type is not found, it reverts with InvalidProofType.
   * @param _publicInput The computed public input hash cast as uint256.
   * @param _verifierAddress The address of the proof type verifier contract.
   * @param _proof The proof to be verified with the proof type verifier contract.
   */
  function _verifyProof(uint256 _publicInput, address _verifierAddress, bytes calldata _proof) internal {
    uint256[] memory publicInput = new uint256[](1);
    publicInput[0] = _publicInput;

    (bool callSuccess, bytes memory result) = _verifierAddress.call(
      abi.encodeCall(IPlonkVerifier.Verify, (_proof, publicInput))
    );

    if (!callSuccess) {
      if (result.length > 0) {
        assembly {
          let dataOffset := add(result, 0x20)

          // Store the modified first 32 bytes back into memory overwriting the location after having swapped out the selector.
          mstore(
            dataOffset,
            or(
              // InvalidProofOrProofVerificationRanOutOfGas(string) = 0xca389c44bf373a5a506ab5a7d8a53cb0ea12ba7c5872fd2bc4a0e31614c00a85.
              shl(224, 0xca389c44),
              and(mload(dataOffset), 0x00000000ffffffffffffffffffffffffffffffffffffffffffffffffffffffff)
            )
          )

          revert(dataOffset, mload(result))
        }
      } else {
        revert InvalidProofOrProofVerificationRanOutOfGas("Unknown");
      }
    }

    bool proofSucceeded = abi.decode(result, (bool));
    if (!proofSucceeded) {
      revert InvalidProof();
    }
  }
}
