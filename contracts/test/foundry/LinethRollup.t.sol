// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import "forge-std/Test.sol";

import { LinethRollup } from "src/rollup/LinethRollup.sol";
import { ILinethRollupBase } from "src/rollup/interfaces/ILinethRollupBase.sol";
import { CalldataBlobAcceptor } from "src/rollup/dataAvailability/CalldataBlobAcceptor.sol";
import { EfficientLeftRightKeccak } from "src/libraries/EfficientLeftRightKeccak.sol";
import { FinalizedStateHashing } from "src/libraries/FinalizedStateHashing.sol";
import { IPlonkVerifier } from "src/verifiers/interfaces/IPlonkVerifier.sol";

import { IPauseManager } from "src/security/pausing/interfaces/IPauseManager.sol";
import { IPermissionsManager } from "src/security/access/interfaces/IPermissionsManager.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import { IntegrationTestTrueVerifier } from "src/_testing/integration/IntegrationTestTrueVerifier.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";

/// @dev Verifier that records the last public input it was asked to verify.
contract RecordingVerifier is IPlonkVerifier {
  uint256 public lastPublicInput;

  function getChainConfiguration() external pure returns (bytes32) {
    return bytes32(0);
  }

  function Verify(bytes calldata, uint256[] calldata _publicInput) external returns (bool) {
    lastPublicInput = _publicInput[0];
    return true;
  }
}

/// @dev Mirrors TestLinethRollup: production LinethRollup plus calldata DA for unit tests.
contract LinethRollupTestHelper is LinethRollup, CalldataBlobAcceptor {
  function computeDataRollingHash(bytes32 _parentDataRollingHash, bytes32 _chunkHash) external pure returns (bytes32) {
    return EfficientLeftRightKeccak._efficientKeccak(_parentDataRollingHash, _chunkHash);
  }

  function setSlotValue(uint256 _slot, uint256 _value) external {
    assembly {
      sstore(_slot, _value)
    }
  }

  function setupParentDataRollingHash(bytes32 _dataRollingHash) external {
    _dataRollingHashExists[_dataRollingHash] = DATA_ROLLING_HASH_EXISTS_DEFAULT_VALUE;
  }

  function setLegacyFinalizedShnarf(bytes32 _legacyShnarf) external {
    currentFinalizedShnarf_DEPRECATED = _legacyShnarf;
  }

  function setRollingHash(uint256 _messageNumber, bytes32 _rollingHash) external {
    rollingHashes[_messageNumber] = _rollingHash;
  }

  function setForcedTransaction(
    uint256 _forcedTransactionNumber,
    bytes32 _rollingHash,
    uint256 _l2BlockNumber
  ) external {
    forcedTransactionRollingHashes[_forcedTransactionNumber] = _rollingHash;
    forcedTransactionL2BlockNumbers[_forcedTransactionNumber] = _l2BlockNumber;
  }

  function setCurrentDataPosition(bytes32 _dataRollingHash, uint256 _dataTailBytes) external {
    currentDataRollingHash = _dataRollingHash;
    currentDataTailBytes = _dataTailBytes;
  }

  function renounceRole(
    bytes32 _role,
    address _account
  ) public virtual override(LinethRollup, AccessControlUpgradeable) {
    super.renounceRole(_role, _account);
  }
}

contract LinethRollupTest is Test {
  LinethRollupTestHelper linethRollup;
  LinethRollupTestHelper implementation;
  address operator;
  address defaultAdmin;
  address verifier;
  address nonAuthorizedAccount;
  address fallbackOperator;
  address yieldManager;

  bytes32 VERIFIER_SETTER_ROLE;
  bytes32 VERIFIER_UNSETTER_ROLE;
  bytes32 OPERATOR_ROLE;
  bytes32 DEFAULT_ADMIN_ROLE;

  bytes32 constant INITIAL_BLOCK_HASH = keccak256("initial-block-hash");
  bytes32 GENESIS_DATA_ROLLING_HASH;

  function setUp() public {
    operator = address(0x1);
    defaultAdmin = address(0x2);
    securityCouncilSetup();
    fallbackOperator = address(0x4);
    nonAuthorizedAccount = address(0x5);
    yieldManager = address(0x6);

    IntegrationTestTrueVerifier trueVerifier = new IntegrationTestTrueVerifier();
    verifier = address(trueVerifier);

    implementation = new LinethRollupTestHelper();

    ILinethRollupBase.BaseInitializationData memory initData;
    initData.initialBlockHash = INITIAL_BLOCK_HASH;
    initData.initialL2BlockNumber = 0;
    initData.genesisTimestamp = 1;
    initData.defaultVerifier = verifier;
    initData.rateLimitPeriodInSeconds = 86400;
    initData.rateLimitAmountInWei = 100 ether;

    initData.roleAddresses = new IPermissionsManager.RoleAddress[](1);
    initData.roleAddresses[0] = IPermissionsManager.RoleAddress({
      addressWithRole: operator,
      role: implementation.OPERATOR_ROLE()
    });

    initData.pauseTypeRoles = new IPauseManager.PauseTypeRole[](0);
    initData.unpauseTypeRoles = new IPauseManager.PauseTypeRole[](0);
    initData.verifierKeys = new bytes32[](0);
    initData.defaultAdmin = defaultAdmin;
    initData.dataRollingHashProvider = address(0);
    initData.addressFilter = defaultAdmin;

    bytes memory initializer = abi.encodeWithSelector(
      LinethRollup.initialize.selector,
      initData,
      fallbackOperator,
      yieldManager
    );

    ERC1967Proxy proxy = new ERC1967Proxy(address(implementation), initializer);

    linethRollup = LinethRollupTestHelper(address(proxy));

    VERIFIER_SETTER_ROLE = linethRollup.VERIFIER_SETTER_ROLE();
    VERIFIER_UNSETTER_ROLE = linethRollup.VERIFIER_UNSETTER_ROLE();
    OPERATOR_ROLE = linethRollup.OPERATOR_ROLE();
    DEFAULT_ADMIN_ROLE = linethRollup.DEFAULT_ADMIN_ROLE();

    GENESIS_DATA_ROLLING_HASH = linethRollup.computeDataRollingHash(bytes32(0), INITIAL_BLOCK_HASH);

    assertEq(linethRollup.hasRole(DEFAULT_ADMIN_ROLE, defaultAdmin), true, "Default admin not set");
    assertEq(linethRollup.hasRole(OPERATOR_ROLE, operator), true, "Operator not set");
  }

  function securityCouncilSetup() internal view {
    // defaultAdmin doubles as security council in these unit tests.
    assertTrue(defaultAdmin != address(0));
  }

  function testSubmitDataAsCalldata() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";

    // Genesis DA stream position: parent dataRollingHash is the deterministic genesis anchor.
    bytes32 parentDataRollingHash = GENESIS_DATA_ROLLING_HASH;
    bytes32 chunkHash = keccak256(compressedData);
    bytes32 storedDataRollingHash = linethRollup.computeDataRollingHash(parentDataRollingHash, chunkHash);

    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, parentDataRollingHash, storedDataRollingHash);

    uint256 exists = linethRollup.dataRollingHashExists(storedDataRollingHash);
    assertEq(exists, 1, "Data rolling hash should be anchored after submission");
  }

  function testFinalizeBlocksHappyPath() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 finalBlockHash = keccak256("final-block-hash");

    // Genesis position: parent dataRollingHash = deterministic genesis anchor, offset = 0.
    bytes32 parentDataRollingHash = GENESIS_DATA_ROLLING_HASH;

    bytes32 chunkHash = keccak256(compressedData);
    bytes32 endDataRollingHash = linethRollup.computeDataRollingHash(parentDataRollingHash, chunkHash);

    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, parentDataRollingHash, endDataRollingHash);

    ILinethRollupBase.FinalizationDataV5 memory finalizationData;
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = finalBlockHash;
    finalizationData.parentDataRollingHash = parentDataRollingHash;
    finalizationData.endDataRollingHash = endDataRollingHash;
    finalizationData.parentDataTailTakeBytes = 0;
    finalizationData.finalDataTailDiscardBytes = 100;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";

    vm.warp(10);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", 0, finalizationData);

    assertEq(linethRollup.blockHashes(10), finalBlockHash, "Final block hash not anchored");
    assertEq(linethRollup.currentL2BlockNumber(), 10, "Current L2 block not updated");
    assertEq(linethRollup.currentDataRollingHash(), endDataRollingHash, "currentDataRollingHash not updated");
    assertEq(linethRollup.currentDataTailBytes(), 100, "currentDataTailBytes not updated");
    assertEq(linethRollup.dataRollingHashExists(GENESIS_DATA_ROLLING_HASH), 1, "genesis dataRollingHash not anchored");
  }

  function testComputeDataRollingHash() public view {
    bytes32 parent = keccak256("parent");
    bytes32 chunk = keccak256("chunk");
    bytes32 expected = keccak256(abi.encodePacked(parent, chunk));
    assertEq(linethRollup.computeDataRollingHash(parent, chunk), expected, "2-input dataRollingHash mismatch");
  }

  function testSubmitDataAsCalldataRevertsOnUnanchoredParent() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 unanchoredParent = keccak256("not-anchored");
    bytes32 endDataRollingHash = linethRollup.computeDataRollingHash(unanchoredParent, keccak256(compressedData));

    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(bytes4(keccak256("ParentDataRollingHashNotAnchored(bytes32)")), unanchoredParent)
    );
    linethRollup.submitDataAsCalldata(compressedData, unanchoredParent, endDataRollingHash);
  }

  function testSubmitDataAsCalldataRevertsOnDataRollingHashMismatch() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 parentDataRollingHash = bytes32(0);
    bytes32 wrongStored = keccak256("wrong");
    // Precompute before the prank so the helper call is not intercepted by expectRevert.
    bytes32 computedDataRollingHash = linethRollup.computeDataRollingHash(
      parentDataRollingHash,
      keccak256(compressedData)
    );

    vm.expectRevert(
      abi.encodeWithSelector(
        bytes4(keccak256("DataRollingHashMismatch(bytes32,bytes32)")),
        wrongStored,
        computedDataRollingHash
      )
    );
    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, parentDataRollingHash, wrongStored);
  }

  // ---------------------------------------------------------------------------------------------------------
  // Finalization defaults: a default final L1 rolling hash / message number signals "no transition": it is bound
  // as-is into the public input, while storage keeps the previous values. Forced transaction values are always the
  // actual values and are stored as passed in.
  // ---------------------------------------------------------------------------------------------------------

  uint256 constant PROOF_TYPE_RECORDING = 7;
  uint256 constant MODULO_R = 21888242871839275222246405745257275088548364400416034343698204186575808495617;

  uint256 constant L1_MESSAGE_NUMBER = 5;
  bytes32 constant L1_ROLLING_HASH = keccak256("l1-rolling-hash");
  uint256 constant FTX_NUMBER = 2;
  bytes32 constant FTX_ROLLING_HASH = keccak256("ftx-rolling-hash");
  bytes32 constant ROUND_1_BLOCK_HASH = keccak256("round-1-block-hash");
  bytes32 constant ROUND_2_BLOCK_HASH = keccak256("round-2-block-hash");
  bytes constant CHUNK_1 = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
  bytes constant CHUNK_2 = hex"ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100";

  function _setupRecordingVerifier() internal returns (RecordingVerifier recordingVerifier) {
    recordingVerifier = new RecordingVerifier();
    vm.startPrank(defaultAdmin);
    linethRollup.grantRole(VERIFIER_SETTER_ROLE, defaultAdmin);
    linethRollup.setVerifierAddress(address(recordingVerifier), PROOF_TYPE_RECORDING);
    vm.stopPrank();
  }

  function _data(
    bytes32 _parentBlockHash,
    uint256 _endBlockNumber,
    uint256 _lastFinalizedTimestamp,
    uint256 _finalTimestamp,
    bytes32 _finalBlockHash,
    bytes32 _parentDataRollingHash,
    bytes32 _endDataRollingHash,
    uint256 _parentTailTake,
    uint256 _finalTailDiscard
  ) internal pure returns (ILinethRollupBase.FinalizationDataV5 memory finalizationData) {
    finalizationData.parentBlockHash = _parentBlockHash;
    finalizationData.endBlockNumber = _endBlockNumber;
    finalizationData.lastFinalizedTimestamp = _lastFinalizedTimestamp;
    finalizationData.finalTimestamp = _finalTimestamp;
    finalizationData.finalBlockHash = _finalBlockHash;
    finalizationData.parentDataRollingHash = _parentDataRollingHash;
    finalizationData.endDataRollingHash = _endDataRollingHash;
    finalizationData.parentDataTailTakeBytes = _parentTailTake;
    finalizationData.finalDataTailDiscardBytes = _finalTailDiscard;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";
  }

  /// @dev Independent re-implementation of the public input layout.
  function _expectedPublicInput(
    ILinethRollupBase.FinalizationDataV5 memory _d,
    uint256 _finalL1Number,
    bytes32 _finalL1Hash,
    uint256 _finalFtxNumber,
    bytes32 _finalFtxHash
  ) internal pure returns (uint256) {
    bytes memory part1 = abi.encodePacked(
      _d.parentBlockHash,
      _d.finalBlockHash,
      _d.finalTimestamp,
      _d.endBlockNumber,
      _d.lastFinalizedL1RollingHash,
      _finalL1Hash,
      _d.lastFinalizedL1RollingHashMessageNumber,
      _finalL1Number
    );
    bytes memory part2 = abi.encodePacked(
      _d.lastFinalizedForcedTransactionRollingHash,
      _finalFtxHash,
      _d.lastFinalizedForcedTransactionNumber,
      _finalFtxNumber,
      _d.l2MerkleTreesDepth
    );
    bytes memory part3 = abi.encodePacked(
      _d.parentDataRollingHash,
      _d.endDataRollingHash,
      _d.parentDataTailTakeBytes,
      _d.finalDataTailDiscardBytes
    );
    // Empty arrays/bytes in all tests: each dynamic field hashes to keccak256("").
    bytes32 emptyHash = keccak256("");
    bytes memory part4 = abi.encodePacked(
      emptyHash,
      bytes32(0) /* chain configuration */,
      emptyHash,
      emptyHash,
      emptyHash
    );
    return uint256(keccak256(abi.encodePacked(part1, part2, part3, part4))) % MODULO_R;
  }

  function _finalizedState(
    uint256 _l1Number,
    bytes32 _l1Hash,
    uint256 _ftxNumber,
    bytes32 _ftxHash,
    uint256 _timestamp
  ) internal pure returns (bytes32) {
    return FinalizedStateHashing._computeLastFinalizedState(_l1Number, _l1Hash, _ftxNumber, _ftxHash, _timestamp);
  }

  /// @dev Round 1 explicitly sets the final L1 rolling hash/message number and the final forced transaction number.
  function _finalizeRound1Explicit() internal returns (bytes32 endDataRollingHash1) {
    linethRollup.setRollingHash(L1_MESSAGE_NUMBER, L1_ROLLING_HASH);
    // Forced transaction 2 is processed (deadline block 5 <= end block 10); forced transaction 3 does not exist yet.
    linethRollup.setForcedTransaction(FTX_NUMBER, FTX_ROLLING_HASH, 5);

    endDataRollingHash1 = linethRollup.computeDataRollingHash(GENESIS_DATA_ROLLING_HASH, keccak256(CHUNK_1));
    vm.prank(operator);
    linethRollup.submitDataAsCalldata(CHUNK_1, GENESIS_DATA_ROLLING_HASH, endDataRollingHash1);

    ILinethRollupBase.FinalizationDataV5 memory d = _data(
      INITIAL_BLOCK_HASH,
      10,
      1,
      2,
      ROUND_1_BLOCK_HASH,
      GENESIS_DATA_ROLLING_HASH,
      endDataRollingHash1,
      0,
      100
    );
    d.l1RollingHashMessageNumber = L1_MESSAGE_NUMBER;
    d.l1RollingHash = L1_ROLLING_HASH;
    d.finalForcedTransactionNumber = FTX_NUMBER;

    vm.warp(10);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);
  }

  /// @dev Round 2 data with the final L1 rolling hash fields left at their defaults and the actual (unchanged) final
  ///   forced transaction number.
  function _round2Data(bytes32 _endDataRollingHash1) internal returns (ILinethRollupBase.FinalizationDataV5 memory d) {
    bytes32 endDataRollingHash2 = linethRollup.computeDataRollingHash(_endDataRollingHash1, keccak256(CHUNK_2));
    vm.prank(operator);
    linethRollup.submitDataAsCalldata(CHUNK_2, _endDataRollingHash1, endDataRollingHash2);

    d = _data(ROUND_1_BLOCK_HASH, 20, 2, 3, ROUND_2_BLOCK_HASH, _endDataRollingHash1, endDataRollingHash2, 100, 0);
    d.lastFinalizedL1RollingHashMessageNumber = L1_MESSAGE_NUMBER;
    d.lastFinalizedL1RollingHash = L1_ROLLING_HASH;
    d.lastFinalizedForcedTransactionNumber = FTX_NUMBER;
    d.lastFinalizedForcedTransactionRollingHash = FTX_ROLLING_HASH;
    d.finalForcedTransactionNumber = FTX_NUMBER;
  }

  function testFinalizeBlocksWithExplicitFinalStateBindsPublicInput() public {
    RecordingVerifier recordingVerifier = _setupRecordingVerifier();
    _finalizeRound1Explicit();

    ILinethRollupBase.FinalizationDataV5 memory d = _data(
      INITIAL_BLOCK_HASH,
      10,
      1,
      2,
      ROUND_1_BLOCK_HASH,
      GENESIS_DATA_ROLLING_HASH,
      linethRollup.currentDataRollingHash(),
      0,
      100
    );

    assertEq(
      linethRollup.currentFinalizedState(),
      _finalizedState(L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER, FTX_ROLLING_HASH, 2),
      "Explicit final state not stored"
    );
    assertEq(
      recordingVerifier.lastPublicInput(),
      _expectedPublicInput(d, L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER, FTX_ROLLING_HASH),
      "Public input layout mismatch"
    );
  }

  function testFinalizeBlocksDefaultsAtGenesisKeepGenesisState() public {
    _setupRecordingVerifier();
    bytes32 endDataRollingHash = linethRollup.computeDataRollingHash(GENESIS_DATA_ROLLING_HASH, keccak256(CHUNK_1));
    vm.prank(operator);
    linethRollup.submitDataAsCalldata(CHUNK_1, GENESIS_DATA_ROLLING_HASH, endDataRollingHash);

    ILinethRollupBase.FinalizationDataV5 memory d = _data(
      INITIAL_BLOCK_HASH,
      10,
      1,
      2,
      ROUND_1_BLOCK_HASH,
      GENESIS_DATA_ROLLING_HASH,
      endDataRollingHash,
      0,
      100
    );

    vm.warp(10);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);

    assertEq(linethRollup.currentFinalizedState(), _finalizedState(0, bytes32(0), 0, bytes32(0), 2));
  }

  function testFinalizeBlocksL1DefaultsCarryForwardLastFinalizedState() public {
    RecordingVerifier recordingVerifier = _setupRecordingVerifier();
    bytes32 endDataRollingHash1 = _finalizeRound1Explicit();
    ILinethRollupBase.FinalizationDataV5 memory d = _round2Data(endDataRollingHash1);

    vm.expectEmit(true, false, false, true, address(linethRollup));
    emit ILinethRollupBase.FinalizedStateUpdated(20, 3, L1_MESSAGE_NUMBER, FTX_NUMBER);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);

    assertEq(
      linethRollup.currentFinalizedState(),
      _finalizedState(L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER, FTX_ROLLING_HASH, 3),
      "Defaults did not carry the last finalized state forward"
    );
    assertEq(linethRollup.currentL2BlockNumber(), 20, "Block number not advanced");
    assertEq(linethRollup.blockHashes(20), ROUND_2_BLOCK_HASH, "Block hash not anchored");

    assertEq(
      recordingVerifier.lastPublicInput(),
      _expectedPublicInput(d, 0, bytes32(0), FTX_NUMBER, FTX_ROLLING_HASH),
      "Public input must bind the L1 defaults as passed in"
    );
  }

  function testFinalizeBlocksExplicitL1PairCarryForwardMatchesDefaults() public {
    RecordingVerifier recordingVerifier = _setupRecordingVerifier();
    bytes32 endDataRollingHash1 = _finalizeRound1Explicit();
    ILinethRollupBase.FinalizationDataV5 memory d = _round2Data(endDataRollingHash1);

    // Re-supplying the already finalized pair explicitly (the prover carries it forward when no new L1 message
    // was anchored) is accepted: equal is fine, only a lower message number reverts.
    d.l1RollingHashMessageNumber = L1_MESSAGE_NUMBER;
    d.l1RollingHash = L1_ROLLING_HASH;

    vm.expectEmit(true, false, false, true, address(linethRollup));
    emit ILinethRollupBase.FinalizedStateUpdated(20, 3, L1_MESSAGE_NUMBER, FTX_NUMBER);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);

    assertEq(
      linethRollup.currentFinalizedState(),
      _finalizedState(L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER, FTX_ROLLING_HASH, 3),
      "Explicit carry-forward must leave the same finalized state as the defaults"
    );
    assertEq(
      recordingVerifier.lastPublicInput(),
      _expectedPublicInput(d, L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER, FTX_ROLLING_HASH),
      "Public input must bind the explicitly carried-forward L1 pair"
    );
  }

  function testFinalizeBlocksDefaultL1KeepsStateWithForcedTransactionTransition() public {
    RecordingVerifier recordingVerifier = _setupRecordingVerifier();
    bytes32 endDataRollingHash1 = _finalizeRound1Explicit();
    ILinethRollupBase.FinalizationDataV5 memory d = _round2Data(endDataRollingHash1);

    // No L1 rolling hash transition (defaults), but forced transaction 3 is processed in this range.
    bytes32 ftx3RollingHash = keccak256("ftx-3");
    linethRollup.setForcedTransaction(FTX_NUMBER + 1, ftx3RollingHash, 15);
    d.finalForcedTransactionNumber = FTX_NUMBER + 1;

    vm.expectEmit(true, false, false, true, address(linethRollup));
    emit ILinethRollupBase.FinalizedStateUpdated(20, 3, L1_MESSAGE_NUMBER, FTX_NUMBER + 1);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);

    assertEq(
      linethRollup.currentFinalizedState(),
      _finalizedState(L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER + 1, ftx3RollingHash, 3),
      "L1 state not kept or forced transaction state not advanced"
    );
    assertEq(
      recordingVerifier.lastPublicInput(),
      _expectedPublicInput(d, 0, bytes32(0), FTX_NUMBER + 1, ftx3RollingHash),
      "Public input must bind the L1 defaults as passed in and the new forced transaction state"
    );
  }

  function testFinalizeBlocksL1DefaultsRevertOnIncorrectLastFinalizedState() public {
    _setupRecordingVerifier();
    bytes32 endDataRollingHash1 = _finalizeRound1Explicit();
    ILinethRollupBase.FinalizationDataV5 memory d = _round2Data(endDataRollingHash1);

    // A wrong carried-forward L1 rolling hash state must not be accepted via defaults.
    d.lastFinalizedL1RollingHashMessageNumber = 0;
    d.lastFinalizedL1RollingHash = bytes32(0);

    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(
        bytes4(keccak256("FinalizationStateIncorrect(bytes32,bytes32)")),
        _finalizedState(L1_MESSAGE_NUMBER, L1_ROLLING_HASH, FTX_NUMBER, FTX_ROLLING_HASH, 2),
        _finalizedState(0, bytes32(0), FTX_NUMBER, FTX_ROLLING_HASH, 2)
      )
    );
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);
  }

  function testFinalizeBlocksRevertsOnPartialL1RollingHashDefaults() public {
    _setupRecordingVerifier();
    bytes32 endDataRollingHash1 = _finalizeRound1Explicit();
    ILinethRollupBase.FinalizationDataV5 memory d = _round2Data(endDataRollingHash1);

    // Message number default with a non-default hash.
    d.l1RollingHash = keccak256("orphan-hash");
    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(bytes4(keccak256("MissingMessageNumberForRollingHash(bytes32)")), keccak256("orphan-hash"))
    );
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);

    // Hash default with a non-default message number.
    d.l1RollingHash = bytes32(0);
    d.l1RollingHashMessageNumber = L1_MESSAGE_NUMBER;
    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(bytes4(keccak256("MissingRollingHashForMessageNumber(uint256)")), L1_MESSAGE_NUMBER)
    );
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);
  }

  function testFinalizeBlocksRevertsOnDecreasingL1RollingHashMessageNumber() public {
    _setupRecordingVerifier();
    bytes32 endDataRollingHash1 = _finalizeRound1Explicit();
    ILinethRollupBase.FinalizationDataV5 memory d = _round2Data(endDataRollingHash1);

    // An older anchored pair (an authentic hash that exists on L1) must not be accepted.
    bytes32 olderRollingHash = keccak256("older-l1-rolling-hash");
    linethRollup.setRollingHash(L1_MESSAGE_NUMBER - 1, olderRollingHash);
    d.l1RollingHashMessageNumber = L1_MESSAGE_NUMBER - 1;
    d.l1RollingHash = olderRollingHash;
    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(
        bytes4(keccak256("L1RollingHashMessageNumberDecreasing(uint256,uint256)")),
        L1_MESSAGE_NUMBER - 1,
        L1_MESSAGE_NUMBER
      )
    );
    linethRollup.finalizeBlocks(hex"01", PROOF_TYPE_RECORDING, d);
  }

  function testFinalizeBlocksRevertsOnUnanchoredEndDataRollingHash() public {
    bytes32 parentDataRollingHash = GENESIS_DATA_ROLLING_HASH;

    ILinethRollupBase.FinalizationDataV5 memory finalizationData;
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = keccak256("final-block-hash");
    finalizationData.parentDataRollingHash = parentDataRollingHash;
    finalizationData.endDataRollingHash = keccak256("never-anchored");
    finalizationData.parentDataTailTakeBytes = 0;
    finalizationData.finalDataTailDiscardBytes = 100;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";

    vm.warp(10);
    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(bytes4(keccak256("FinalDataRollingHashNotAnchored(bytes32)")), keccak256("never-anchored"))
    );
    linethRollup.finalizeBlocks(hex"01", 0, finalizationData);
  }

  function testFinalizeBlocksRevertsOnDataRollingHashNotContinuous() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 endDataRollingHash = linethRollup.computeDataRollingHash(
      GENESIS_DATA_ROLLING_HASH,
      keccak256(compressedData)
    );

    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, GENESIS_DATA_ROLLING_HASH, endDataRollingHash);

    ILinethRollupBase.FinalizationDataV5 memory finalizationData;
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = keccak256("final-block-hash");
    // Wrong parent: does not match the live currentDataRollingHash (deliberately wrong value).
    finalizationData.parentDataRollingHash = keccak256("wrong-prev");
    finalizationData.endDataRollingHash = endDataRollingHash;
    finalizationData.parentDataTailTakeBytes = 0;
    finalizationData.finalDataTailDiscardBytes = 100;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";

    vm.warp(10);
    vm.prank(operator);
    vm.expectRevert(
      abi.encodeWithSelector(
        bytes4(keccak256("DataRollingHashNotContinuous(bytes32,bytes32)")),
        GENESIS_DATA_ROLLING_HASH,
        keccak256("wrong-prev")
      )
    );
    linethRollup.finalizeBlocks(hex"01", 0, finalizationData);
  }

  function testReinitializeLineaRollupV10AlreadyInitializedReverts() public {
    // setUp() already ran `initialize`, which itself uses reinitializer(10); calling
    // reinitializeLineaRollupV10 again must revert since version 10 was already consumed on this proxy.
    vm.expectRevert(); // InvalidInitialization
    linethRollup.reinitializeLineaRollupV10();
  }

  function testReinitializeLineaRollupV10MigratesLegacyShnarf() public {
    // Simulate a pre-v10 upgrade: lower the initialized version so reinitializer(10) can run again.
    linethRollup.setSlotValue(0, 9);

    bytes32 legacyShnarf = keccak256("legacy-shnarf-to-migrate");
    linethRollup.setLegacyFinalizedShnarf(legacyShnarf);

    linethRollup.reinitializeLineaRollupV10();

    assertEq(
      linethRollup.currentFinalizedShnarf_DEPRECATED(),
      bytes32(0),
      "Legacy shnarf not wiped after reinitializer migration"
    );
    assertEq(linethRollup.currentDataRollingHash(), legacyShnarf, "currentDataRollingHash not migrated");
    assertEq(linethRollup.dataRollingHashExists(legacyShnarf), 1, "migrated legacy shnarf not anchored");
  }

  function testReinitializeLineaRollupV10EmitsLegacyShnarfMigrated() public {
    linethRollup.setSlotValue(0, 9);

    bytes32 legacyShnarf = keccak256("legacy-shnarf-for-event");
    linethRollup.setLegacyFinalizedShnarf(legacyShnarf);

    vm.expectEmit(true, false, false, false);
    emit ILinethRollupBase.LegacyShnarfMigrated(legacyShnarf);
    linethRollup.reinitializeLineaRollupV10();
  }

  function testChangeVerifierNotAuthorized() public {
    address newVerifier = address(0x1234);

    vm.prank(nonAuthorizedAccount);
    vm.expectRevert(
      abi.encodePacked(
        "AccessControl: account ",
        _toAsciiString(nonAuthorizedAccount),
        " is missing role ",
        _toHexString(VERIFIER_SETTER_ROLE)
      )
    );
    linethRollup.setVerifierAddress(newVerifier, 2);
  }

  function testSetVerifierAddressSuccess() public {
    vm.startPrank(defaultAdmin);
    linethRollup.grantRole(VERIFIER_SETTER_ROLE, defaultAdmin);
    vm.stopPrank();

    address newVerifier = address(0x1234);

    vm.prank(defaultAdmin);
    linethRollup.setVerifierAddress(newVerifier, 2);

    assertEq(linethRollup.verifiers(2), newVerifier, "Verifier address not updated");
  }

  function testUnsetVerifierAddress() public {
    vm.startPrank(defaultAdmin);
    linethRollup.grantRole(VERIFIER_UNSETTER_ROLE, defaultAdmin);
    linethRollup.grantRole(VERIFIER_SETTER_ROLE, defaultAdmin);

    address newVerifier = address(0x1234);
    linethRollup.setVerifierAddress(newVerifier, 0);
    vm.stopPrank();

    vm.prank(defaultAdmin);
    linethRollup.unsetVerifierAddress(0);

    assertEq(linethRollup.verifiers(0), address(0), "Verifier address not unset");
  }

  function testUnsetVerifierNotAuthorized() public {
    vm.prank(nonAuthorizedAccount);
    vm.expectRevert(
      abi.encodePacked(
        "AccessControl: account ",
        _toAsciiString(nonAuthorizedAccount),
        " is missing role ",
        _toHexString(VERIFIER_UNSETTER_ROLE)
      )
    );
    linethRollup.unsetVerifierAddress(0);
  }

  function _toAsciiString(address x) internal pure returns (string memory) {
    bytes memory s = new bytes(42);
    s[0] = "0";
    s[1] = "x";
    for (uint256 i = 0; i < 20; i++) {
      uint8 b = uint8(uint256(uint160(x)) / (2 ** (8 * (19 - i))));
      uint8 hi = b / 16;
      uint8 lo = b - 16 * hi;
      s[2 + 2 * i] = _char(hi);
      s[3 + 2 * i] = _char(lo);
    }
    return string(s);
  }

  function _char(uint8 b) internal pure returns (bytes1 c) {
    if (b < 10) {
      return bytes1(b + 0x30);
    } else {
      return bytes1(b + 0x57);
    }
  }

  function _toHexString(bytes32 data) internal pure returns (string memory) {
    return _toHexString(abi.encodePacked(data));
  }

  function _toHexString(bytes memory data) internal pure returns (string memory) {
    bytes memory hexString = new bytes(data.length * 2 + 2);
    hexString[0] = "0";
    hexString[1] = "x";
    bytes memory hexChars = "0123456789abcdef";
    for (uint256 i = 0; i < data.length; i++) {
      hexString[2 + i * 2] = hexChars[uint8(data[i] >> 4)];
      hexString[3 + i * 2] = hexChars[uint8(data[i] & 0x0f)];
    }
    return string(hexString);
  }
}
