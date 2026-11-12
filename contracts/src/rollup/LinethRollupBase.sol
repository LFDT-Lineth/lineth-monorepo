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
 * @author Consensys Incorporated
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

  /// @notice Value indicating a dataRollingHash is anchored in `_dataRollingHashExists`.
  uint256 internal constant DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE = 1;

  /// @notice Maximum valid `parentDataTailTakeBytes`/`finalDataTailDiscardBytes` value: a blob's
  ///   254-bit-packed usable payload length, minus one. Calldata submissions always use 0.
  uint256 internal constant MAX_DATA_TAIL_BYTES = 130046;

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
   * @notice DEPRECATED. Retained in its original storage slot to hold the last legacy shnarf for the
   *   one-time legacy-shnarf migration in `_finalizeBlocks`.
   * @dev Wiped to EMPTY_HASH once migrated; never written again. Do not use for new logic.
   */
  bytes32 public currentFinalizedShnarf_DEPRECATED;

  /**
   * @dev The only mapping used for data submission tracking. Keys are anchored dataRollingHash
   *   values (previously shnarfs); only the final dataRollingHash of each submission is anchored.
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
   * @dev Directly readable on-chain so the coordinator can determine where to build the next
   *   submission/finalization from. EMPTY_HASH with `currentDataTailBytes == 0` marks a
   *   fresh network or one awaiting the one-time legacy-shnarf bridge (see `currentFinalizedShnarf_DEPRECATED`).
   * @dev Appended at the end of the layout (taken from `__gap_LinethRollup`) to avoid shifting existing slots.
   */
  bytes32 public currentDataRollingHash;

  /**
   * @notice The last finalization's `finalDataTailDiscardBytes`, paired with `currentDataRollingHash`.
   *   The next finalization's `parentDataTailTakeBytes` must equal this value.
   * @dev Always 0 for calldata-based submissions; only EIP-4844 blob submissions advance it.
   */
  uint256 public currentDataTailBytes;

  /// @dev Keep 46 free storage slots for inheriting contracts (reduced from 50 for the four new slots above:
  ///   `verifierKeys`, `blockHashes`, `currentDataRollingHash`, `currentDataTailBytes`).
  uint256[46] private __gap_LinethRollup;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LinethRollup and underlying service dependencies - used for new networks only.
   * @dev `currentDataRollingHash` is deterministically seeded from `initialBlockHash` and anchored into
   *   `_dataRollingHashExists`. `currentDataTailBytes`/`currentFinalizedShnarf_DEPRECATED` are
   *   left at zero — fresh networks have no legacy shnarf to migrate from.
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

    bytes32 genesisDataRollingHash = EfficientLeftRightKeccak._efficientKeccak(
      EMPTY_HASH,
      _initializationData.initialBlockHash
    );
    currentDataRollingHash = genesisDataRollingHash;
    _dataRollingHashExists[genesisDataRollingHash] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;

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
   * @notice Migrates the legacy finalized shnarf into the blob-spanning dataRollingHash model.
   * @dev Sets `currentDataRollingHash` to the still-live legacy finalized shnarf and anchors it so the first
   *   post-upgrade submission and finalization chain directly from it. `currentDataTailBytes` stays 0 (legacy
   *   finalizations always ended on a chunk boundary). The deprecated slot is then wiped.
   * @dev Trusts the on-chain `currentFinalizedShnarf_DEPRECATED` value directly: it was itself the proven output of
   *   the prior contract version's `finalizeBlocks` (or its genesis seed).
   * @dev On any real in-place upgrade the legacy shnarf is never EMPTY_HASH (prior versions seed a genesis shnarf at
   *   initialization). The zero check guards against running the migration on a proxy that never held a legacy
   *   shnarf, which would otherwise anchor EMPTY_HASH as a valid parent.
   * @dev Must only be called from a version-gated reinitializer so it runs exactly once per proxy.
   */
  function _migrateLegacyShnarf() internal {
    bytes32 migratedDataRollingHash = currentFinalizedShnarf_DEPRECATED;
    require(migratedDataRollingHash != EMPTY_HASH, IGenericErrors.ZeroHashNotAllowed());

    currentDataRollingHash = migratedDataRollingHash;
    _dataRollingHashExists[migratedDataRollingHash] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;
    currentFinalizedShnarf_DEPRECATED = EMPTY_HASH;

    emit LegacyShnarfMigrated(migratedDataRollingHash);
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
   * @dev Block-hash anchoring replaced state-root anchoring. If blockHashes[_lastFinalizedBlock] is EMPTY_HASH
   *   the parent was finalized under the legacy state-root model, so this is the one-time migration round:
   *   `_finalizationData.parentBlockHash` must also be EMPTY_HASH and the guest program treats the round as a
   *   migration, binding the legacy parent state root in the proof rather than a parent block hash.
   * @dev Every round after the migration must supply the non-zero parent block hash anchored by the previous
   *   finalization, and every finalization (including the migration round) must anchor a non-zero final block hash.
   * @dev Validation order is deliberate: calldata-only checks (L1 rolling hash pairing and monotonicity, filtered
   *   addresses) run first, then the caller-supplied last finalized tuple is authenticated against
   *   `currentFinalizedState`, and only then are the remaining storage reads performed.
   * @param _finalizationData The full finalization data.
   * @param _lastFinalizedBlock The last finalized block number.
   * @param _finalForcedTransactionRollingHash The rolling hash for the final forced transaction.
   */
  function _finalizeBlocks(
    FinalizationDataV5 calldata _finalizationData,
    uint256 _lastFinalizedBlock,
    bytes32 _finalForcedTransactionRollingHash
  ) internal {
    _validateL2ComputedRollingHash(
      _finalizationData.l1RollingHashMessageNumber,
      _finalizationData.l1RollingHash,
      _finalizationData.lastFinalizedL1RollingHashMessageNumber
    );

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

    // EMPTY_HASH signals the one-time migration path: parent was committed under the old state-root-hash
    // model, so there is no parent block hash to soft-check against. Every round after relies on the
    // block hash anchored below.
    bytes32 parentBlockHash = blockHashes[_lastFinalizedBlock];

    if (parentBlockHash == EMPTY_HASH) {
      require(_finalizationData.parentBlockHash == EMPTY_HASH, StartingBlockHashDoesNotMatch());
    } else {
      require(parentBlockHash == _finalizationData.parentBlockHash, StartingBlockHashDoesNotMatch());
    }

    require(_finalizationData.finalBlockHash != EMPTY_HASH, FinalizationBlockHashIsZeroHash());

    // Checked as a pair against currentDataRollingHash/currentDataTailBytes: the same
    // dataRollingHash can recur at different tail-byte counts, so both must match exactly.
    require(
      _finalizationData.parentDataRollingHash == currentDataRollingHash,
      DataRollingHashNotContinuous(currentDataRollingHash, _finalizationData.parentDataRollingHash)
    );

    require(
      _finalizationData.parentDataTailTakeBytes == currentDataTailBytes,
      ParentDataTailTakeBytesNotContinuous(currentDataTailBytes, _finalizationData.parentDataTailTakeBytes)
    );

    // Both tail-byte counts must lie within a single blob chunk's unpacked payload. Calldata-based
    // submissions are open-ended in length and always carry a tail-byte count of 0, so they satisfy
    // this trivially; blob submissions are bounded by the blob's 254-bit-packed usable byte length.
    require(
      _finalizationData.parentDataTailTakeBytes <= MAX_DATA_TAIL_BYTES,
      DataTailBytesOutOfRange(_finalizationData.parentDataTailTakeBytes)
    );
    require(
      _finalizationData.finalDataTailDiscardBytes <= MAX_DATA_TAIL_BYTES,
      DataTailBytesOutOfRange(_finalizationData.finalDataTailDiscardBytes)
    );

    // Same-dataRollingHash guard: no new chunk anchored, so this finalization only advances further
    // into an already-shared chunk. Forward progress requires parentDataTailTakeBytes >
    // finalDataTailDiscardBytes (both measure bytes from the same chunk's tail).
    if (_finalizationData.parentDataRollingHash == _finalizationData.endDataRollingHash) {
      require(
        _finalizationData.parentDataTailTakeBytes > _finalizationData.finalDataTailDiscardBytes,
        InvalidDataTailProgression(
          _finalizationData.parentDataTailTakeBytes,
          _finalizationData.finalDataTailDiscardBytes
        )
      );
    }

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
    currentDataTailBytes = _finalizationData.finalDataTailDiscardBytes;
    _updateFinalizedState(_finalizationData, _finalForcedTransactionRollingHash);

    unchecked {
      emit DataFinalizedV4(
        ++_lastFinalizedBlock,
        _finalizationData.endBlockNumber,
        _finalizationData.endDataRollingHash,
        _finalizationData.finalDataTailDiscardBytes,
        parentBlockHash,
        _finalizationData.finalBlockHash
      );
    }
  }

  /**
   * @notice Stores the new finalized state and emits FinalizedStateUpdated, keeping the previous L1 rolling hash
   *   values when they are passed as defaults.
   * @dev A default L1 rolling hash pair (`l1RollingHashMessageNumber == 0` with `l1RollingHash == EMPTY_HASH`,
   *   enforced by `_validateL2ComputedRollingHash`) signals "no transition" (the guest program(s) infer the same), so
   *   `lastFinalizedL1RollingHashMessageNumber`/`lastFinalizedL1RollingHash` are kept. Only the public input uses the
   *   values exactly as passed in.
   * @dev Forced transaction values are always stored as passed in.
   * @dev The emitted message number always matches `currentFinalizedState`, as off-chain consumers (e.g. forced
   *   transaction submitters) rebuild the last finalized state from the latest event.
   * @dev The `lastFinalized*` values were validated against `currentFinalizedState` earlier in `_finalizeBlocks`.
   * @param _finalizationData The full finalization data.
   * @param _finalForcedTransactionRollingHash The rolling hash for the final forced transaction.
   */
  function _updateFinalizedState(
    FinalizationDataV5 calldata _finalizationData,
    bytes32 _finalForcedTransactionRollingHash
  ) internal {
    uint256 l1RollingHashMessageNumber = _finalizationData.l1RollingHashMessageNumber;
    bytes32 l1RollingHash = _finalizationData.l1RollingHash;
    if (l1RollingHashMessageNumber == 0) {
      l1RollingHashMessageNumber = _finalizationData.lastFinalizedL1RollingHashMessageNumber;
      l1RollingHash = _finalizationData.lastFinalizedL1RollingHash;
    }

    currentFinalizedState = FinalizedStateHashing._computeLastFinalizedState(
      l1RollingHashMessageNumber,
      l1RollingHash,
      _finalizationData.finalForcedTransactionNumber,
      _finalForcedTransactionRollingHash,
      _finalizationData.finalTimestamp
    );

    emit FinalizedStateUpdated(
      _finalizationData.endBlockNumber,
      _finalizationData.finalTimestamp,
      l1RollingHashMessageNumber,
      _finalizationData.finalForcedTransactionNumber
    );
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
   * @notice Validates the L2 computed L1 rolling hash pair against L1 storage.
   * @dev A zero message number must be paired with an empty rolling hash (no transition in this range).
   * @dev A non-zero message number must be paired with a non-empty rolling hash that matches `rollingHashes`.
   * @dev Like the forced transaction number, the message number may stay equal to the last finalized one
   *   (the prover carries the parent aggregation's pair forward when no new L1 message was anchored) or
   *   advance, but it may never go backwards. The guest program already enforces this; the explicit on-chain
   *   check is an additional safeguard for finalization paths that do not rely on a zk proof (e.g. signed
   *   attestations).
   * @dev Ordering: the calldata-only checks (zero pairing and monotonicity) run before the `rollingHashes`
   *   state read, and `_lastFinalizedRollingHashMessageNumber` is caller-supplied and only authenticated against
   *   `currentFinalizedState` later in `_finalizeBlocks`, so a stale last-finalized tuple may surface here first.
   *   Any mismatch reverts the whole finalization either way.
   * @param _rollingHashMessageNumber Message number associated with the rolling hash as computed on L2.
   * @param _rollingHash L1 rolling hash as computed on L2.
   * @param _lastFinalizedRollingHashMessageNumber The last finalized L1 rolling hash message number.
   */
  function _validateL2ComputedRollingHash(
    uint256 _rollingHashMessageNumber,
    bytes32 _rollingHash,
    uint256 _lastFinalizedRollingHashMessageNumber
  ) internal view {
    if (_rollingHashMessageNumber == 0) {
      if (_rollingHash != EMPTY_HASH) {
        revert MissingMessageNumberForRollingHash(_rollingHash);
      }
    } else {
      if (_rollingHash == EMPTY_HASH) {
        revert MissingRollingHashForMessageNumber(_rollingHashMessageNumber);
      }
      if (_rollingHashMessageNumber < _lastFinalizedRollingHashMessageNumber) {
        revert L1RollingHashMessageNumberDecreasing(_rollingHashMessageNumber, _lastFinalizedRollingHashMessageNumber);
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
   * @dev Assembly is cheaper gas wise; the dynamic-sized fields are placed last in `_finalizationData`
   *   to optimise hashing ranges.
   * @dev Binds the blob-spanning public-input surface (see
   *   `docs/workflows/operations/blobSubmissionAndFinalization.md`) directly from the plain
   *   `parentDataRollingHash`/`endDataRollingHash`/`parentDataTailTakeBytes`/`finalDataTailDiscardBytes`
   *   fields, rather than the opaque shnarf commitments used previously. Execution continuity is bound via
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
   *     _finalizationData.parentDataTailTakeBytes,
   *     _finalizationData.finalDataTailDiscardBytes,
   *     keccak256(
   *         abi.encodePacked(_finalizationData.l2MerkleRoots)
   *     ),
   *     _verifierChainConfiguration,
   *     keccak256(abi.encodePacked(_finalizationData.filteredAddresses)),
   *     keccak256(abi.encodePacked(_finalizationData.verifierKeys)),
   *     keccak256(abi.encodePacked(_finalizationData.l2MessagingBlocksOffsets))
   *   )
   * )
   * Data is found at the following offsets:
   * 0x00    parentBlockHash
   * 0x20    endBlockNumber
   * 0x40    lastFinalizedTimestamp
   * 0x60    finalTimestamp
   * 0x80    lastFinalizedL1RollingHash
   * 0xa0    l1RollingHash
   * 0xc0    lastFinalizedL1RollingHashMessageNumber
   * 0xe0    l1RollingHashMessageNumber
   * 0x100   l2MerkleTreesDepth
   * 0x120   lastFinalizedForcedTransactionNumber
   * 0x140   finalForcedTransactionNumber
   * 0x160   lastFinalizedForcedTransactionRollingHash
   * 0x180   finalBlockHash
   * 0x1a0   parentDataRollingHash
   * 0x1c0   endDataRollingHash
   * 0x1e0   parentDataTailTakeBytes
   * 0x200   finalDataTailDiscardBytes
   * 0x220   l2MerkleRootsLengthLocation
   * 0x240   filteredAddressesLengthLocation
   * 0x260   verifierKeysLengthLocation
   * 0x280   l2MessagingBlocksOffsetsLengthLocation
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
    bytes32 hashedL2MessagingBlocksOffsets = keccak256(abi.encodePacked(_finalizationData.l2MessagingBlocksOffsets));

    assembly {
      let mPtr := mload(0x40)

      /**
       * _finalizationData.parentBlockHash
       * _finalizationData.finalBlockHash
       */
      mstore(mPtr, calldataload(_finalizationData))
      mstore(add(mPtr, 0x20), calldataload(add(_finalizationData, 0x180)))

      /**
       * _finalizationData.finalTimestamp
       * _finalizationData.endBlockNumber
       */
      mstore(add(mPtr, 0x40), calldataload(add(_finalizationData, 0x60)))
      mstore(add(mPtr, 0x60), calldataload(add(_finalizationData, 0x20)))

      /**
       * _finalizationData.lastFinalizedL1RollingHash
       * _finalizationData.l1RollingHash
       * _finalizationData.lastFinalizedL1RollingHashMessageNumber
       * _finalizationData.l1RollingHashMessageNumber
       */
      calldatacopy(add(mPtr, 0x80), add(_finalizationData, 0x80), 0x80)

      // lastFinalizedForcedTransactionRollingHash
      mstore(add(mPtr, 0x100), calldataload(add(_finalizationData, 0x160)))

      // finalForcedTransactionRollingHash
      mstore(add(mPtr, 0x120), _finalForcedTransactionRollingHash)

      /**
       * _finalizationData.lastFinalizedForcedTransactionNumber
       * _finalizationData.finalForcedTransactionNumber
       */
      calldatacopy(add(mPtr, 0x140), add(_finalizationData, 0x120), 0x40)

      // _finalizationData.l2MerkleTreesDepth
      mstore(add(mPtr, 0x180), calldataload(add(_finalizationData, 0x100)))

      /**
       * _finalizationData.parentDataRollingHash
       * _finalizationData.endDataRollingHash
       * _finalizationData.parentDataTailTakeBytes
       * _finalizationData.finalDataTailDiscardBytes
       */
      calldatacopy(add(mPtr, 0x1a0), add(_finalizationData, 0x1a0), 0x80)

      mstore(add(mPtr, 0x220), hashedL2MerkleRoots)
      mstore(add(mPtr, 0x240), _verifierChainConfiguration)
      mstore(add(mPtr, 0x260), hashedFilteredAddresses)
      mstore(add(mPtr, 0x280), hashedVerifierKeys)
      mstore(add(mPtr, 0x2a0), hashedL2MessagingBlocksOffsets)

      publicInput := mod(keccak256(mPtr, 0x2c0), MODULO_R)
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
