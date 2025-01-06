// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.26;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L1MessageService } from "./messageService/l1/L1MessageService.sol";
import { ZkEvmV2 } from "./ZkEvmV2.sol";
import { ILineaRollup } from "./interfaces/l1/ILineaRollup.sol";
import { PermissionsManager } from "./lib/PermissionsManager.sol";

import { Utils } from "./lib/Utils.sol";
/**
 * @title Contract to manage cross-chain messaging on L1, L2 data submission, and rollup proof verification.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LineaRollup is
  Initializable,
  AccessControlUpgradeable,
  ZkEvmV2,
  L1MessageService,
  PermissionsManager,
  ILineaRollup
{
  using Utils for *;

  /// @notice This is the ABI version and not the reinitialize version.
  string public constant CONTRACT_VERSION = "6.0";

  /// @notice The role required to set/add  proof verifiers by type.
  bytes32 public constant VERIFIER_SETTER_ROLE = keccak256("VERIFIER_SETTER_ROLE");

  /// @notice The role required to set/remove  proof verifiers by type.
  bytes32 public constant VERIFIER_UNSETTER_ROLE = keccak256("VERIFIER_UNSETTER_ROLE");

  /// @dev Value indicating a shnarf exists.
  uint256 internal constant SHNARF_EXISTS_DEFAULT_VALUE = 1;

  /// @dev The default hash value.
  bytes32 internal constant EMPTY_HASH = 0x0;

  /// @dev The BLS Curve modulus value used.
  uint256 internal constant BLS_CURVE_MODULUS =
    52435875175126190479447740508185965837690552500527637822603658699938581184513;

  /// @dev The well-known precompile address for point evaluation.
  address internal constant POINT_EVALUATION_PRECOMPILE_ADDRESS = address(0x0a);

  /// @dev The expected point evaluation return data length.
  uint256 internal constant POINT_EVALUATION_RETURN_DATA_LENGTH = 64;

  /// @dev The expected point evaluation field element length returned.
  uint256 internal constant POINT_EVALUATION_FIELD_ELEMENTS_LENGTH = 4096;

  /// @dev In practice, when used, this is expected to be a close approximation to 6 months, and is intentional.
  uint256 internal constant SIX_MONTHS_IN_SECONDS = (365 / 2) * 24 * 60 * 60;

  /// @dev DEPRECATED in favor of the single blobShnarfExists mapping.
  mapping(bytes32 dataHash => bytes32 finalStateRootHash) public dataFinalStateRootHashes;
  /// @dev DEPRECATED in favor of the single blobShnarfExists mapping.
  mapping(bytes32 dataHash => bytes32 parentHash) public dataParents;
  /// @dev DEPRECATED in favor of the single blobShnarfExists mapping.
  mapping(bytes32 dataHash => bytes32 shnarfHash) public dataShnarfHashes;
  /// @dev DEPRECATED in favor of the single blobShnarfExists mapping.
  mapping(bytes32 dataHash => uint256 startingBlock) public dataStartingBlock;
  /// @dev DEPRECATED in favor of the single blobShnarfExists mapping.
  mapping(bytes32 dataHash => uint256 endingBlock) public dataEndingBlock;

  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  uint256 public currentL2StoredL1MessageNumber;
  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  bytes32 public currentL2StoredL1RollingHash;

  /// @notice Contains the most recent finalized shnarf.
  bytes32 public currentFinalizedShnarf;

  /**
   * @dev NB: THIS IS THE ONLY MAPPING BEING USED FOR DATA SUBMISSION TRACKING.
   * @dev NB: This was shnarfFinalBlockNumbers and is replaced to indicate only that a shnarf exists with a value of 1.
   */
  mapping(bytes32 shnarf => uint256 exists) public blobShnarfExists;

  /// @notice Hash of the L2 computed L1 message number, rolling hash and finalized timestamp.
  bytes32 public currentFinalizedState;

  /// @notice The address of the fallback operator.
  /// @dev This address is granted the OPERATOR_ROLE after six months of finalization inactivity by the current operators.
  address public fallbackOperator;

  /// @dev Total contract storage is 11 slots.

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes LineaRollup and underlying service dependencies - used for new networks only.
   * @dev DEFAULT_ADMIN_ROLE is set for the security council.
   * @dev OPERATOR_ROLE is set for operators.
   * @dev Note: This is used for new testnets and local/CI testing, and will not replace existing proxy based contracts.
   * @param _initializationData The initial data used for proof verification.
   */
  function initialize(InitializationData calldata _initializationData) external initializer {
    if (_initializationData.defaultVerifier == address(0)) {
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

    if (_initializationData.fallbackOperator == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    fallbackOperator = _initializationData.fallbackOperator;
    emit FallbackOperatorAddressSet(msg.sender, _initializationData.fallbackOperator);

    currentL2BlockNumber = _initializationData.initialL2BlockNumber;
    stateRootHashes[_initializationData.initialL2BlockNumber] = _initializationData.initialStateRootHash;

    bytes32 genesisShnarf = _computeShnarf(
      EMPTY_HASH,
      EMPTY_HASH,
      _initializationData.initialStateRootHash,
      EMPTY_HASH,
      EMPTY_HASH
    );

    blobShnarfExists[genesisShnarf] = SHNARF_EXISTS_DEFAULT_VALUE;
    currentFinalizedShnarf = genesisShnarf;
    currentFinalizedState = _computeLastFinalizedState(0, EMPTY_HASH, _initializationData.genesisTimestamp);
  }

  /**
   * @notice Sets permissions for a list of addresses and their roles as well as initialises the PauseManager pauseType:role mappings and fallback operator.
   * @dev This function is a reinitializer and can only be called once per version. Should be called using an upgradeAndCall transaction to the ProxyAdmin.
   * @param _roleAddresses The list of addresses and roles to assign permissions to.
   * @param _pauseTypeRoles The list of pause types to associate with roles.
   * @param _unpauseTypeRoles The list of unpause types to associate with roles.
   * @param _fallbackOperator The address of the fallback operator.
   */
  function reinitializeLineaRollupV6(
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles,
    address _fallbackOperator
  ) external reinitializer(6) {
    __Permissions_init(_roleAddresses);
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);

    if (_fallbackOperator == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    fallbackOperator = _fallbackOperator;
    emit FallbackOperatorAddressSet(msg.sender, _fallbackOperator);

    /// @dev using the constants requires string memory and more complex code.
    emit LineaRollupVersionChanged(bytes8("5.0"), bytes8("6.0"));
  }

  /**
   * @notice Revokes `role` from the calling account.
   * @dev Fallback operator cannot renounce role. Reverts with OnlyNonFallbackOperator.
   * @param _role The role to renounce.
   * @param _account The account to renounce - can only be the _msgSender().
   */
  function renounceRole(bytes32 _role, address _account) public override {
    if (_account == fallbackOperator) {
      revert OnlyNonFallbackOperator();
    }

    super.renounceRole(_role, _account);
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
   * @notice Sets the fallback operator role to the specified address if six months have passed since the last finalization.
   * @dev Reverts if six months have not passed since the last finalization.
   * @param _messageNumber Last finalized L1 message number as part of the feedback loop.
   * @param _rollingHash Last finalized L1 rolling hash as part of the feedback loop.
   * @param _lastFinalizedTimestamp Last finalized L2 block timestamp.
   */
  function setFallbackOperator(uint256 _messageNumber, bytes32 _rollingHash, uint256 _lastFinalizedTimestamp) external {
    if (block.timestamp < _lastFinalizedTimestamp + SIX_MONTHS_IN_SECONDS) {
      revert LastFinalizationTimeNotLapsed();
    }
    if (currentFinalizedState != _computeLastFinalizedState(_messageNumber, _rollingHash, _lastFinalizedTimestamp)) {
      revert FinalizationStateIncorrect(
        currentFinalizedState,
        _computeLastFinalizedState(_messageNumber, _rollingHash, _lastFinalizedTimestamp)
      );
    }

    address fallbackOperatorAddress = fallbackOperator;

    _grantRole(OPERATOR_ROLE, fallbackOperatorAddress);
    emit FallbackOperatorRoleGranted(msg.sender, fallbackOperatorAddress);
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
   * @notice Submit one or more EIP-4844 blobs.
   * @dev OPERATOR_ROLE is required to execute.
   * @dev This should be a blob carrying transaction.
   * @param _blobSubmissions The data for blob submission including proofs and required polynomials.
   * @param _parentShnarf The parent shnarf used in continuity checks as it includes the parentStateRootHash in its computation.
   * @param _finalBlobShnarf The expected final shnarf post computation of all the blob shnarfs.
   */
  function submitBlobs(
    BlobSubmission[] calldata _blobSubmissions,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
  ) external whenTypeAndGeneralNotPaused(PauseType.BLOB_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    if (_blobSubmissions.length == 0) {
      revert BlobSubmissionDataIsMissing();
    }

    if (blobhash(_blobSubmissions.length) != EMPTY_HASH) {
      revert BlobSubmissionDataEmpty(_blobSubmissions.length);
    }

    if (blobShnarfExists[_parentShnarf] == 0) {
      revert ParentBlobNotSubmitted(_parentShnarf);
    }

    /**
     * @dev validate we haven't submitted the last shnarf. There is a final check at the end of the function verifying,
     * that _finalBlobShnarf was computed correctly.
     * Note: As only the last shnarf is stored, we don't need to validate shnarfs,
     * computed for any previous blobs in the submission (if multiple are submitted).
     */
    if (blobShnarfExists[_finalBlobShnarf] != 0) {
      revert DataAlreadySubmitted(_finalBlobShnarf);
    }

    bytes32 currentDataEvaluationPoint;
    bytes32 currentDataHash;

    /// @dev Assigning in memory saves a lot of gas vs. calldata reading.
    BlobSubmission memory blobSubmission;

    bytes32 computedShnarf = _parentShnarf;

    for (uint256 i; i < _blobSubmissions.length; i++) {
      blobSubmission = _blobSubmissions[i];

      currentDataHash = blobhash(i);

      if (currentDataHash == EMPTY_HASH) {
        revert EmptyBlobDataAtIndex(i);
      }

      bytes32 snarkHash = blobSubmission.snarkHash;

      currentDataEvaluationPoint = Utils._efficientKeccak(snarkHash, currentDataHash);

      _verifyPointEvaluation(
        currentDataHash,
        uint256(currentDataEvaluationPoint),
        blobSubmission.dataEvaluationClaim,
        blobSubmission.kzgCommitment,
        blobSubmission.kzgProof
      );

      computedShnarf = _computeShnarf(
        computedShnarf,
        snarkHash,
        blobSubmission.finalStateRootHash,
        currentDataEvaluationPoint,
        bytes32(blobSubmission.dataEvaluationClaim)
      );
    }

    if (_finalBlobShnarf != computedShnarf) {
      revert FinalShnarfWrong(_finalBlobShnarf, computedShnarf);
    }

    /// @dev use the last shnarf as the submission to store as technically it becomes the next parent shnarf.
    blobShnarfExists[computedShnarf] = SHNARF_EXISTS_DEFAULT_VALUE;

    emit DataSubmittedV3(_parentShnarf, computedShnarf, blobSubmission.finalStateRootHash);
  }

  /**
   * @notice Submit blobs using compressed data via calldata.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _submission The supporting data for compressed data submission including compressed data.
   * @param _parentShnarf The parent shnarf used in continuity checks as it includes the parentStateRootHash in its computation.
   * @param _expectedShnarf The expected shnarf post computation of all the submission.
   */
  function submitDataAsCalldata(
    CompressedCalldataSubmission calldata _submission,
    bytes32 _parentShnarf,
    bytes32 _expectedShnarf
  ) external whenTypeAndGeneralNotPaused(PauseType.CALLDATA_SUBMISSION) onlyRole(OPERATOR_ROLE) {
    if (_submission.compressedData.length == 0) {
      revert EmptySubmissionData();
    }

    if (blobShnarfExists[_expectedShnarf] != 0) {
      revert DataAlreadySubmitted(_expectedShnarf);
    }

    if (blobShnarfExists[_parentShnarf] == 0) {
      revert ParentBlobNotSubmitted(_parentShnarf);
    }

    bytes32 currentDataHash = keccak256(_submission.compressedData);

    bytes32 dataEvaluationPoint = Utils._efficientKeccak(_submission.snarkHash, currentDataHash);

    bytes32 computedShnarf = _computeShnarf(
      _parentShnarf,
      _submission.snarkHash,
      _submission.finalStateRootHash,
      dataEvaluationPoint,
      _calculateY(_submission.compressedData, dataEvaluationPoint)
    );

    if (_expectedShnarf != computedShnarf) {
      revert FinalShnarfWrong(_expectedShnarf, computedShnarf);
    }

    blobShnarfExists[computedShnarf] = SHNARF_EXISTS_DEFAULT_VALUE;

    emit DataSubmittedV3(_parentShnarf, computedShnarf, _submission.finalStateRootHash);
  }

  /**
   * @notice Internal function to compute and save the finalization state.
   * @dev Using assembly this way is cheaper gas wise.
   * @param _messageNumber Is the last L2 computed L1 message number in the finalization.
   * @param _rollingHash Is the last L2 computed L1 rolling hash in the finalization.
   * @param _timestamp The final timestamp in the finalization.
   */
  function _computeLastFinalizedState(
    uint256 _messageNumber,
    bytes32 _rollingHash,
    uint256 _timestamp
  ) internal pure returns (bytes32 hashedFinalizationState) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _messageNumber)
      mstore(add(mPtr, 0x20), _rollingHash)
      mstore(add(mPtr, 0x40), _timestamp)
      hashedFinalizationState := keccak256(mPtr, 0x60)
    }
  }

  /**
   * @notice Internal function to compute the shnarf more efficiently.
   * @dev Using assembly this way is cheaper gas wise.
   * @param _parentShnarf The shnarf of the parent data item.
   * @param _snarkHash Is the computed hash for compressed data (using a SNARK-friendly hash function) that aggregates per data submission to be used in public input.
   * @param _finalStateRootHash The final state root hash of the data being submitted.
   * @param _dataEvaluationPoint The data evaluation point.
   * @param _dataEvaluationClaim The data evaluation claim.
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
   * @notice Performs point evaluation for the compressed blob.
   * @dev _dataEvaluationPoint is modular reduced to be lower than the BLS_CURVE_MODULUS for precompile checks.
   * @param _currentDataHash The current blob versioned hash.
   * @param _dataEvaluationPoint The data evaluation point.
   * @param _dataEvaluationClaim The data evaluation claim.
   * @param _kzgCommitment The blob KZG commitment.
   * @param _kzgProof The blob KZG point proof.
   */
  function _verifyPointEvaluation(
    bytes32 _currentDataHash,
    uint256 _dataEvaluationPoint,
    uint256 _dataEvaluationClaim,
    bytes memory _kzgCommitment,
    bytes memory _kzgProof
  ) internal view {
    assembly {
      _dataEvaluationPoint := mod(_dataEvaluationPoint, BLS_CURVE_MODULUS)
    }

    (bool success, bytes memory returnData) = POINT_EVALUATION_PRECOMPILE_ADDRESS.staticcall(
      abi.encodePacked(_currentDataHash, _dataEvaluationPoint, _dataEvaluationClaim, _kzgCommitment, _kzgProof)
    );

    if (!success) {
      revert PointEvaluationFailed();
    }

    if (returnData.length != POINT_EVALUATION_RETURN_DATA_LENGTH) {
      revert PrecompileReturnDataLengthWrong(POINT_EVALUATION_RETURN_DATA_LENGTH, returnData.length);
    }

    uint256 fieldElements;
    uint256 blsCurveModulus;
    assembly {
      fieldElements := mload(add(returnData, 0x20))
      blsCurveModulus := mload(add(returnData, POINT_EVALUATION_RETURN_DATA_LENGTH))
    }
    if (fieldElements != POINT_EVALUATION_FIELD_ELEMENTS_LENGTH || blsCurveModulus != BLS_CURVE_MODULUS) {
      revert PointEvaluationResponseInvalid(fieldElements, blsCurveModulus);
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
    FinalizationDataV3 calldata _finalizationData
  ) external whenTypeAndGeneralNotPaused(PauseType.FINALIZATION) onlyRole(OPERATOR_ROLE) {
    if (_aggregatedProof.length == 0) {
      revert ProofIsEmpty();
    }

    uint256 lastFinalizedBlockNumber = currentL2BlockNumber;

    if (stateRootHashes[lastFinalizedBlockNumber] != _finalizationData.parentStateRootHash) {
      revert StartingRootHashDoesNotMatch();
    }

    /// @dev currentFinalizedShnarf is updated in _finalizeBlocks and lastFinalizedShnarf MUST be set beforehand for the transition.
    bytes32 lastFinalizedShnarf = currentFinalizedShnarf;

    bytes32 finalShnarf = _finalizeBlocks(_finalizationData, lastFinalizedBlockNumber);

    uint256 publicInput = _computePublicInput(
      _finalizationData,
      lastFinalizedShnarf,
      finalShnarf,
      lastFinalizedBlockNumber,
      _finalizationData.endBlockNumber
    );

    _verifyProof(publicInput, _proofType, _aggregatedProof);
  }

  /**
   * @notice Internal function to finalize compressed blocks.
   * @param _finalizationData The full finalization data.
   * @param _lastFinalizedBlock The last finalized block.
   * @return finalShnarf The final computed shnarf in finalizing.
   */
  function _finalizeBlocks(
    FinalizationDataV3 calldata _finalizationData,
    uint256 _lastFinalizedBlock
  ) internal returns (bytes32 finalShnarf) {
    if (_finalizationData.endBlockNumber <= _lastFinalizedBlock) {
      revert FinalBlockNumberLessThanOrEqualToLastFinalizedBlock(_finalizationData.endBlockNumber, _lastFinalizedBlock);
    }

    _validateL2ComputedRollingHash(_finalizationData.l1RollingHashMessageNumber, _finalizationData.l1RollingHash);

    if (
      _computeLastFinalizedState(
        _finalizationData.lastFinalizedL1RollingHashMessageNumber,
        _finalizationData.lastFinalizedL1RollingHash,
        _finalizationData.lastFinalizedTimestamp
      ) != currentFinalizedState
    ) {
      revert FinalizationStateIncorrect(
        _computeLastFinalizedState(
          _finalizationData.lastFinalizedL1RollingHashMessageNumber,
          _finalizationData.lastFinalizedL1RollingHash,
          _finalizationData.lastFinalizedTimestamp
        ),
        currentFinalizedState
      );
    }

    if (_finalizationData.finalTimestamp >= block.timestamp) {
      revert FinalizationInTheFuture(_finalizationData.finalTimestamp, block.timestamp);
    }

    if (_finalizationData.shnarfData.finalStateRootHash == EMPTY_HASH) {
      revert FinalBlockStateEqualsZeroHash();
    }

    finalShnarf = _computeShnarf(
      _finalizationData.shnarfData.parentShnarf,
      _finalizationData.shnarfData.snarkHash,
      _finalizationData.shnarfData.finalStateRootHash,
      _finalizationData.shnarfData.dataEvaluationPoint,
      _finalizationData.shnarfData.dataEvaluationClaim
    );

    if (blobShnarfExists[finalShnarf] == 0) {
      revert FinalBlobNotSubmitted(finalShnarf);
    }

    _addL2MerkleRoots(_finalizationData.l2MerkleRoots, _finalizationData.l2MerkleTreesDepth);
    _anchorL2MessagingBlocks(_finalizationData.l2MessagingBlocksOffsets, _lastFinalizedBlock);

    stateRootHashes[_finalizationData.endBlockNumber] = _finalizationData.shnarfData.finalStateRootHash;

    currentL2BlockNumber = _finalizationData.endBlockNumber;

    currentFinalizedShnarf = finalShnarf;

    currentFinalizedState = _computeLastFinalizedState(
      _finalizationData.l1RollingHashMessageNumber,
      _finalizationData.l1RollingHash,
      _finalizationData.finalTimestamp
    );

    emit DataFinalizedV3(
      ++_lastFinalizedBlock,
      _finalizationData.endBlockNumber,
      finalShnarf,
      _finalizationData.parentStateRootHash,
      _finalizationData.shnarfData.finalStateRootHash
    );
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
   * @notice Internal function to calculate Y for public input generation.
   * @param _data Compressed data from submission data.
   * @param _dataEvaluationPoint The data evaluation point.
   * @dev Each chunk of 32 bytes must start with a 0 byte.
   * @dev The dataEvaluationPoint value is modulo-ed down during the computation and scalar field checking is not needed.
   * @dev There is a hard constraint in the circuit to enforce the polynomial degree limit (4096), which will also be enforced with EIP-4844.
   * @return compressedDataComputedY The Y calculated value using the Horner method.
   */
  function _calculateY(
    bytes calldata _data,
    bytes32 _dataEvaluationPoint
  ) internal pure returns (bytes32 compressedDataComputedY) {
    if (_data.length % 0x20 != 0) {
      revert BytesLengthNotMultipleOf32();
    }

    bytes4 errorSelector = ILineaRollup.FirstByteIsNotZero.selector;
    assembly {
      for {
        let i := _data.length
      } gt(i, 0) {

      } {
        i := sub(i, 0x20)
        let chunk := calldataload(add(_data.offset, i))
        if iszero(iszero(and(chunk, 0xFF00000000000000000000000000000000000000000000000000000000000000))) {
          let ptr := mload(0x40)
          mstore(ptr, errorSelector)
          revert(ptr, 0x4)
        }
        compressedDataComputedY := addmod(
          mulmod(compressedDataComputedY, _dataEvaluationPoint, BLS_CURVE_MODULUS),
          chunk,
          BLS_CURVE_MODULUS
        )
      }
    }
  }

  /**
   * @notice Compute the public input.
   * @dev Using assembly this way is cheaper gas wise.
   * @dev NB: the dynamic sized fields are placed last in _finalizationData on purpose to optimise hashing ranges.
   * @dev Computing the public input as the following:
   * keccak256(
   *  abi.encode(
   *     _lastFinalizedShnarf,
   *     _finalShnarf,
   *     _finalizationData.lastFinalizedTimestamp,
   *     _finalizationData.finalTimestamp,
   *     _lastFinalizedBlockNumber,
   *     _finalizationData.endBlockNumber,
   *     _finalizationData.lastFinalizedL1RollingHash,
   *     _finalizationData.l1RollingHash,
   *     _finalizationData.lastFinalizedL1RollingHashMessageNumber,
   *     _finalizationData.l1RollingHashMessageNumber,
   *     _finalizationData.l2MerkleTreesDepth,
   *     keccak256(
   *         abi.encodePacked(_finalizationData.l2MerkleRoots)
   *     )
   *   )
   * )
   * Data is found at the following offsets:
   * 0x00    parentStateRootHash
   * 0x20    endBlockNumber
   * 0x40    shnarfData.parentShnarf
   * 0x60    shnarfData.snarkHash
   * 0x80    shnarfData.finalStateRootHash
   * 0xa0    shnarfData.dataEvaluationPoint
   * 0xc0    shnarfData.dataEvaluationClaim
   * 0xe0    lastFinalizedTimestamp
   * 0x100   finalTimestamp
   * 0x120   lastFinalizedL1RollingHash
   * 0x140   l1RollingHash
   * 0x160   lastFinalizedL1RollingHashMessageNumber
   * 0x180   l1RollingHashMessageNumber
   * 0x1a0   l2MerkleTreesDepth
   * 0x1c0   l2MerkleRootsLengthLocation
   * 0x1e0   l2MessagingBlocksOffsetsLengthLocation
   * Dynamic l2MerkleRootsLength
   * Dynamic l2MerkleRoots
   * Dynamic l2MessagingBlocksOffsetsLength (location depends on where l2MerkleRoots ends)
   * Dynamic l2MessagingBlocksOffsets (location depends on where l2MerkleRoots ends)
   * @param _finalizationData The full finalization data.
   * @param _finalShnarf The final shnarf in the finalization.
   * @param _lastFinalizedBlockNumber The last finalized block number.
   * @param _endBlockNumber End block number being finalized.
   */
  function _computePublicInput(
    FinalizationDataV3 calldata _finalizationData,
    bytes32 _lastFinalizedShnarf,
    bytes32 _finalShnarf,
    uint256 _lastFinalizedBlockNumber,
    uint256 _endBlockNumber
  ) private pure returns (uint256 publicInput) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _lastFinalizedShnarf)
      mstore(add(mPtr, 0x20), _finalShnarf)

      /**
       * _finalizationData.lastFinalizedTimestamp
       * _finalizationData.finalTimestamp
       */
      calldatacopy(add(mPtr, 0x40), add(_finalizationData, 0xe0), 0x40)

      mstore(add(mPtr, 0x80), _lastFinalizedBlockNumber)

      // _finalizationData.endBlockNumber
      mstore(add(mPtr, 0xA0), _endBlockNumber)

      /**
       * _finalizationData.lastFinalizedL1RollingHash
       * _finalizationData.l1RollingHash
       * _finalizationData.lastFinalizedL1RollingHashMessageNumber
       * _finalizationData.l1RollingHashMessageNumber
       * _finalizationData.l2MerkleTreesDepth
       */
      calldatacopy(add(mPtr, 0xC0), add(_finalizationData, 0x120), 0xA0)

      /**
       * @dev Note the following in hashing the _finalizationData.l2MerkleRoots array:
       * The second memory pointer and free pointer are offset by 0x20 to temporarily hash the array outside the scope of working memory,
       * as we need the space left for the array hash to be stored at 0x160.
       */
      let mPtrMerkleRoot := add(mPtr, 0x180)
      let merkleRootsLengthLocation := add(_finalizationData, calldataload(add(_finalizationData, 0x1c0)))
      let merkleRootsLen := calldataload(merkleRootsLengthLocation)
      calldatacopy(mPtrMerkleRoot, add(merkleRootsLengthLocation, 0x20), mul(merkleRootsLen, 0x20))
      let l2MerkleRootsHash := keccak256(mPtrMerkleRoot, mul(merkleRootsLen, 0x20))
      mstore(add(mPtr, 0x160), l2MerkleRootsHash)

      publicInput := mod(keccak256(mPtr, 0x180), MODULO_R)
    }
  }
}
