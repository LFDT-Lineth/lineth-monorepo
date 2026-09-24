// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L1MessageService } from "../messaging/l1/L1MessageService.sol";
import { ZkEvmV2 } from "./ZkEvmV2.sol";
import { ILinethRollupBase } from "./interfaces/ILinethRollupBase.sol";
import { IProvideDataRollingHash } from "./dataAvailability/interfaces/IProvideDataRollingHash.sol";
import { PermissionsManager } from "../security/access/PermissionsManager.sol";
import { IPlonkVerifier } from "../verifiers/interfaces/IPlonkVerifier.sol";
import { FinalizedStateHashing } from "../libraries/FinalizedStateHashing.sol";
import { EfficientLeftRightKeccak } from "../libraries/EfficientLeftRightKeccak.sol";
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
  IProvideDataRollingHash
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

  /// @dev DEPRECATED in favor of the single _dataRollingHashExists mapping.
  mapping(bytes32 dataHash => bytes32 finalStateRootHash) private dataFinalStateRootHashes_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _dataRollingHashExists mapping.
  mapping(bytes32 dataHash => bytes32 parentHash) private dataParents_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _dataRollingHashExists mapping.
  mapping(bytes32 dataHash => bytes32 shnarfHash) private dataShnarfHashes_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _dataRollingHashExists mapping.
  mapping(bytes32 dataHash => uint256 startingBlock) private dataStartingBlock_DEPRECATED;
  /// @dev DEPRECATED in favor of the single _dataRollingHashExists mapping.
  mapping(bytes32 dataHash => uint256 endingBlock) private dataEndingBlock_DEPRECATED;

  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  uint256 private currentL2StoredL1MessageNumber_DEPRECATED;
  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  bytes32 private currentL2StoredL1RollingHash_DEPRECATED;

  /**
   * @notice DEPRECATED as the live DA position tracker. Retained in its original storage slot to
   *   hold the last legacy shnarf (same slot as `main`, `CONTRACT_VERSION() == "8.0"`) for the
   *   one-time legacy-shnarf migration in `_finalizeBlocks`.
   * @dev Wiped to EMPTY_HASH once migrated, and never written again.
   * @dev DEPRECATED. Retained only for the one-time legacy-shnarf migration; do not use for new logic.
   */
  bytes32 public currentFinalizedShnarf_DEPRECATED;

  /**
   * @dev NB: THIS IS THE ONLY MAPPING BEING USED FOR DATA SUBMISSION TRACKING.
   * @dev NB: Keys are anchored dataRollingHash values (previously shnarfs). Only the final
   *   dataRollingHash of each submission is anchored; intermediate chunk folds are not persisted.
   *   Membership-only — execution continuity no longer travels with the DA accumulator.
   */
  mapping(bytes32 dataRollingHash => uint256 exists) internal _dataRollingHashExists;

  /**
   * @notice Hash of the L2 computed message number, its rolling hash,
   * forced transaction number and its rolling hash,
   * and the L2 block timestamp.
   */
  bytes32 public currentFinalizedState;

  /// @notice The address of the liveness recovery operator.
  /// @dev This address is granted the OPERATOR_ROLE after six months of finalization inactivity by the current operators.
  address public livenessRecoveryOperator;

  /// @notice The address of the dataRollingHash provider.
  /// @dev Default is address(this).
  IProvideDataRollingHash public dataRollingHashProvider;

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

  /**
   * @notice The current live end dataRollingHash of the finalized DA stream position.
   * @dev Directly readable on-chain (no opaque commitment to open) so the coordinator can always
   *   determine where to build the next submission/finalization from. EMPTY_HASH combined with
   *   `currentOffset == 0` marks either a genuinely fresh network or a network still awaiting its
   *   one-time legacy-shnarf bridge (see `currentFinalizedShnarf_DEPRECATED`).
   * @dev Storage-layout note: appended at the end of the existing layout (consuming two slots from
   *   `__gap_LineaRollup`) rather than inserted among earlier declarations, to avoid shifting any
   *   already-deployed storage slot.
   */
  bytes32 public currentDataRollingHash;

  /**
   * @notice The current live end offset (bytes consumed of the last-folded chunk) of the finalized
   *   DA stream position, paired with `currentDataRollingHash`.
   */
  uint256 public currentDataAvailabilityOffset;

  /// @dev Keep 46 free storage slots for inheriting contracts (reduced from 48 to account for the two
  ///   new slots above: currentDataRollingHash and currentDataAvailabilityOffset).
  uint256[46] private __gap_LineaRollup;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LinethRollup and underlying service dependencies - used for new networks only.
   * @dev `currentDataRollingHash`/`currentDataAvailabilityOffset`/`currentFinalizedShnarf_DEPRECATED` are left at
   *   their zero defaults — fresh networks have no legacy shnarf to migrate from.
   * @param _initializationData The initial data used for contract initialization.
   */
  function __LinethRollup_init(BaseInitializationData calldata _initializationData) internal virtual onlyInitializing {
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

    currentFinalizedState = FinalizedStateHashing._computeLastFinalizedState(
      0,
      EMPTY_HASH,
      0,
      EMPTY_HASH,
      _initializationData.genesisTimestamp
    );

    nextForcedTransactionNumber = 1;

    address dataRollingHashProviderAddress = _initializationData.dataRollingHashProvider;

    if (dataRollingHashProviderAddress == address(0)) {
      dataRollingHashProviderAddress = address(this);
    }

    dataRollingHashProvider = IProvideDataRollingHash(dataRollingHashProviderAddress);

    addressFilter = IAddressFilter(_initializationData.addressFilter);

    // Seed the initial allowed verifier keys.
    for (uint256 i; i < _initializationData.verifierKeys.length; i++) {
      require(_initializationData.verifierKeys[i] != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());
      verifierKeys[_initializationData.verifierKeys[i]] = true;
    }
    if (_initializationData.verifierKeys.length > 0) {
      emit VerifierKeysSet(_initializationData.verifierKeys);
    }

    emit LineaRollupBaseInitialized(bytes8(bytes(CONTRACT_VERSION())), _initializationData, EMPTY_HASH);
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
   * @notice Computes the legacy 5-input shnarf, used solely by the one-time legacy-shnarf migration
   *   in `_finalizeBlocks`.
   * @dev Same formula and field layout as `main`'s `_computeShnarf`/`ShnarfData`
   *   (`CONTRACT_VERSION() == "8.0"`, see `contracts/deployments/bytecode/2026-07-29`).
   * @dev Using assembly this way is cheaper gas wise.
   * @param _parentShnarf The shnarf of the parent legacy data item.
   * @param _snarkHash The snark hash of the legacy data item.
   * @param _finalStateRootHash The final state root hash of the legacy data item.
   * @param _dataEvaluationPoint The KZG point-evaluation point (z) of the legacy data item.
   * @param _dataEvaluationClaim The KZG point-evaluation claim of the legacy data item.
   * @return shnarf The computed legacy shnarf.
   */
  function _computeShnarf(
    bytes32 _parentShnarf,
    bytes32 _snarkHash,
    bytes32 _finalStateRootHash,
    bytes32 _dataEvaluationPoint,
    bytes32 _dataEvaluationClaim
  ) internal pure returns (bytes32 shnarf) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _parentShnarf)
      mstore(add(mPtr, 0x20), _snarkHash)
      mstore(add(mPtr, 0x40), _finalStateRootHash)
      mstore(add(mPtr, 0x60), _dataEvaluationPoint)
      mstore(add(mPtr, 0x80), _dataEvaluationClaim)
      shnarf := keccak256(mPtr, 0xA0)
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
    FinalizationDataV5 calldata _finalizationData
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

    _finalizeBlocks(_finalizationData, lastFinalizedBlockNumber, finalForcedTransactionRollingHash);

    _verifyProof(
      _computePublicInput(
        _finalizationData,
        finalForcedTransactionRollingHash,
        IPlonkVerifier(verifier).getChainConfiguration()
      ),
      verifier,
      _aggregatedProof
    );
  }

  /**
   * @notice Internal function to finalize compressed blocks.
   * @dev If blockHashes[lastFinalizedBlock] is EMPTY_HASH, validates the legacy parent state root
   *   (block-hash/state-root migration).
   * @dev If `_finalizationData.shnarfData` is non-empty, also runs the one-time legacy-shnarf ->
   *   dataRollingHash migration (see `_computeShnarf`), guarded by `LegacyShnarfAlreadyMigrated`.
   * @param _finalizationData The full finalization data.
   * @param _lastFinalizedBlock The last finalized block number.
   * @param _finalForcedTransactionRollingHash The rolling hash for the final forced transaction.
   */
  function _finalizeBlocks(
    FinalizationDataV5 calldata _finalizationData,
    uint256 _lastFinalizedBlock,
    bytes32 _finalForcedTransactionRollingHash
  ) internal {
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

    // EMPTY_HASH signals the migration path: parent was committed under the old state-root-hash model.
    bytes32 parentBlockHash = blockHashes[_lastFinalizedBlock];

    if (parentBlockHash == EMPTY_HASH) {
      require(_finalizationData.parentBlockHash == EMPTY_HASH, StartingBlockHashDoesNotMatch());
      bytes32 parentStateRootHash = stateRootHashes[_lastFinalizedBlock];
      require(
        parentStateRootHash != EMPTY_HASH && parentStateRootHash == _finalizationData.parentStateRootHash,
        StartingRootHashDoesNotMatch()
      );
    } else {
      require(parentBlockHash == _finalizationData.parentBlockHash, StartingBlockHashDoesNotMatch());
    }

    require(_finalizationData.finalBlockHash != EMPTY_HASH, FinalizationBlockHashIsZeroHash());

    // One-time legacy shnarf -> dataRollingHash migration. Non-empty shnarfData selects this path.
    if (
      _finalizationData.shnarfData.parentShnarf != EMPTY_HASH ||
      _finalizationData.shnarfData.snarkHash != EMPTY_HASH ||
      _finalizationData.shnarfData.finalStateRootHash != EMPTY_HASH ||
      _finalizationData.shnarfData.blobHash != EMPTY_HASH ||
      _finalizationData.shnarfData.dataEvaluationClaim != EMPTY_HASH
    ) {
      require(
        currentDataRollingHash == EMPTY_HASH && currentDataAvailabilityOffset == 0,
        LegacyShnarfAlreadyMigrated()
      );

      // dataEvaluationPoint = keccak256(snarkHash || blobHash), matching
      // Eip4844BlobAcceptor._submitBlobs on `main`.
      bytes32 reconstructedShnarf = _computeShnarf(
        _finalizationData.shnarfData.parentShnarf,
        _finalizationData.shnarfData.snarkHash,
        _finalizationData.shnarfData.finalStateRootHash,
        EfficientLeftRightKeccak._efficientKeccak(
          _finalizationData.shnarfData.snarkHash,
          _finalizationData.shnarfData.blobHash
        ),
        _finalizationData.shnarfData.dataEvaluationClaim
      );

      require(
        reconstructedShnarf == currentFinalizedShnarf_DEPRECATED,
        LegacyShnarfMismatch(currentFinalizedShnarf_DEPRECATED, reconstructedShnarf)
      );

      // Wipe the legacy value — the migration is one-way and can never be repeated.
      currentFinalizedShnarf_DEPRECATED = EMPTY_HASH;

      // Seed the new dataRollingHash chain from the bridged legacy shnarf (offset 0, fresh-start
      // semantics) instead of EMPTY_HASH, explicitly tying post-migration continuity to the
      // pre-migration state root rather than an arbitrary reset point. Anchoring it permanently
      // into the membership set lets the first post-upgrade submission chain from it (see the
      // matching check in `DataRollingHashAcceptorBase._acceptDataRollingHash`).
      currentDataRollingHash = reconstructedShnarf;
      _dataRollingHashExists[reconstructedShnarf] = 1;

      emit LegacyShnarfMigrated(reconstructedShnarf, _finalizationData.shnarfData.blobHash);
    }

    // Checked as a pair against currentDataRollingHash/currentDataAvailabilityOffset: the same
    // dataRollingHash can recur at different offsets, so both must match exactly.
    require(
      _finalizationData.parentDataRollingHash == currentDataRollingHash,
      DataRollingHashNotContinuous(currentDataRollingHash, _finalizationData.parentDataRollingHash)
    );

    require(
      _finalizationData.startOffset == currentDataAvailabilityOffset,
      StartOffsetNotContinuous(currentDataAvailabilityOffset, _finalizationData.startOffset)
    );

    // DA anchoring: the end dataRollingHash must have been anchored by a prior submission.
    require(
      dataRollingHashProvider.dataRollingHashExists(_finalizationData.endDataRollingHash) != 0,
      FinalDataRollingHashNotAnchored(_finalizationData.endDataRollingHash)
    );

    _addL2MerkleRoots(_finalizationData.l2MerkleRoots, _finalizationData.l2MerkleTreesDepth);
    _anchorL2MessagingBlocks(_finalizationData.l2MessagingBlocksOffsets, _lastFinalizedBlock);

    // Anchor the new block hash — moves the next finalization round onto the new path.
    blockHashes[_finalizationData.endBlockNumber] = _finalizationData.finalBlockHash;

    currentL2BlockNumber = _finalizationData.endBlockNumber;
    currentDataRollingHash = _finalizationData.endDataRollingHash;
    currentDataAvailabilityOffset = _finalizationData.endOffset;
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
   * @dev Using assembly this way is cheaper gas wise.
   * @dev NB: the dynamic sized fields are placed last in _finalizationData on purpose to optimise hashing ranges.
   * @dev Binds the full blob-spanning public-input surface (see
   *   `docs/workflows/operations/blobSubmissionAndFinalization.md`) directly from the plain
   *   `parentDataRollingHash`/`endDataRollingHash`/`startOffset`/`endOffset` fields, rather than the
   *   opaque shnarf commitments used previously. Execution-rooting continuity is instead bound via
   *   `parentBlockHash`/`finalBlockHash`, occupying the position the shnarf pair previously held.
   * @dev IMPORTANT: this changes the public input byte layout vs. the prior version and MUST be
   *   mirrored bit-for-bit by the off-chain prover/circuit — a required, coordinated follow-up
   *   outside this repo.
   * @dev Computing the public input as the following:
   * keccak256(
   *  abi.encode(
   *     _finalizationData.parentBlockHash,
   *     _finalizationData.finalBlockHash,
   *     _finalizationData.finalTimestamp,
   *     _finalizationData.endBlockNumber,
   *     _finalizationData.lastFinalizedL1RollingHash,
   *     _finalizationData.l1RollingHash,
   *     _finalizationData.lastFinalizedL1RollingHashMessageNumber,
   *     _finalizationData.l1RollingHashMessageNumber,
   *     _finalizationData.lastFinalizedForcedTransactionRollingHash
   *     _finalForcedTransactionRollingHash,
   *     _finalizationData.lastFinalizedForcedTransactionNumber
   *     _finalizationData.finalForcedTransactionNumber
   *     _finalizationData.l2MerkleTreesDepth,
   *     _finalizationData.parentDataRollingHash,
   *     _finalizationData.endDataRollingHash,
   *     _finalizationData.startOffset,
   *     _finalizationData.endOffset,
   *     keccak256(
   *         abi.encodePacked(_finalizationData.l2MerkleRoots)
   *     ),
   *     _verifierChainConfiguration,
   *     keccak256(abi.encodePacked(_finalizationData.filteredAddresses)),
   *     keccak256(abi.encodePacked(_finalizationData.verifierKeys))
   *   )
   * )
   * Data is found at the following offsets:
   * 0x00    parentStateRootHash
   * 0x20    parentBlockHash
   * 0x40    endBlockNumber
   * 0x60    lastFinalizedTimestamp
   * 0x80    finalTimestamp
   * 0xa0    lastFinalizedL1RollingHash
   * 0xc0    l1RollingHash
   * 0xe0    lastFinalizedL1RollingHashMessageNumber
   * 0x100   l1RollingHashMessageNumber
   * 0x120   l2MerkleTreesDepth
   * 0x140   lastFinalizedForcedTransactionNumber
   * 0x160   finalForcedTransactionNumber
   * 0x180   lastFinalizedForcedTransactionRollingHash
   * 0x1a0   finalBlockHash
   * 0x1c0   parentDataRollingHash
   * 0x1e0   endDataRollingHash
   * 0x200   startOffset
   * 0x220   endOffset
   * 0x240   shnarfData.parentShnarf (legacy migration only, not part of the public input)
   * 0x260   shnarfData.snarkHash
   * 0x280   shnarfData.finalStateRootHash
   * 0x2a0   shnarfData.blobHash
   * 0x2c0   shnarfData.dataEvaluationClaim
   * 0x2e0   l2MerkleRootsLengthLocation
   * 0x300   filteredAddressesLengthLocation
   * 0x320   verifierKeysLengthLocation
   * 0x340   l2MessagingBlocksOffsetsLengthLocation
   * Dynamic l2MerkleRootsLength
   * Dynamic l2MerkleRoots
   * Dynamic filteredAddressesLength
   * Dynamic filteredAddresses
   * Dynamic verifierKeysLength
   * Dynamic verifierKeys
   * Dynamic l2MessagingBlocksOffsetsLength (location depends on where verifierKeys ends)
   * Dynamic l2MessagingBlocksOffsets (location depends on where verifierKeys ends)
   * @param _finalizationData The full finalization data.
   * @param _finalForcedTransactionRollingHash The final processed forced transactions's rolling hash.
   * @param _verifierChainConfiguration The verifier chain configuration.
   * @return publicInput The computed public input.
   */
  function _computePublicInput(
    FinalizationDataV5 calldata _finalizationData,
    bytes32 _finalForcedTransactionRollingHash,
    bytes32 _verifierChainConfiguration
  ) private pure returns (uint256 publicInput) {
    bytes32 hashedL2MerkleRoots = keccak256(abi.encodePacked(_finalizationData.l2MerkleRoots));
    bytes32 hashedFilteredAddresses = keccak256(abi.encodePacked(_finalizationData.filteredAddresses));
    bytes32 hashedVerifierKeys = keccak256(abi.encodePacked(_finalizationData.verifierKeys));

    assembly {
      let mPtr := mload(0x40)

      /**
       * _finalizationData.parentBlockHash
       * _finalizationData.finalBlockHash
       */
      mstore(mPtr, calldataload(add(_finalizationData, 0x20)))
      mstore(add(mPtr, 0x20), calldataload(add(_finalizationData, 0x1a0)))

      /**
       * _finalizationData.finalTimestamp
       * _finalizationData.endBlockNumber
       */
      mstore(add(mPtr, 0x40), calldataload(add(_finalizationData, 0x80)))
      mstore(add(mPtr, 0x60), calldataload(add(_finalizationData, 0x40)))

      /**
       * _finalizationData.lastFinalizedL1RollingHash
       * _finalizationData.l1RollingHash
       * _finalizationData.lastFinalizedL1RollingHashMessageNumber
       * _finalizationData.l1RollingHashMessageNumber
       */
      calldatacopy(add(mPtr, 0x80), add(_finalizationData, 0xa0), 0x80)

      // lastFinalizedForcedTransactionRollingHash
      mstore(add(mPtr, 0x100), calldataload(add(_finalizationData, 0x180)))

      // finalForcedTransactionRollingHash
      mstore(add(mPtr, 0x120), _finalForcedTransactionRollingHash)

      /**
       * _finalizationData.lastFinalizedForcedTransactionNumber
       * _finalizationData.finalForcedTransactionNumber
       */
      calldatacopy(add(mPtr, 0x140), add(_finalizationData, 0x140), 0x40)

      // _finalizationData.l2MerkleTreesDepth
      mstore(add(mPtr, 0x180), calldataload(add(_finalizationData, 0x120)))

      /**
       * _finalizationData.parentDataRollingHash
       * _finalizationData.endDataRollingHash
       * _finalizationData.startOffset
       * _finalizationData.endOffset
       */
      calldatacopy(add(mPtr, 0x1a0), add(_finalizationData, 0x1c0), 0x80)

      mstore(add(mPtr, 0x220), hashedL2MerkleRoots)
      mstore(add(mPtr, 0x240), _verifierChainConfiguration)
      mstore(add(mPtr, 0x260), hashedFilteredAddresses)
      mstore(add(mPtr, 0x280), hashedVerifierKeys)

      publicInput := mod(keccak256(mPtr, 0x2a0), MODULO_R)
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
