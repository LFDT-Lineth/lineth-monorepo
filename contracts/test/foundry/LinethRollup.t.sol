// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import "forge-std/Test.sol";

import { LinethRollup } from "src/rollup/LinethRollup.sol";
import { ILinethRollupBase } from "src/rollup/interfaces/ILinethRollupBase.sol";
import { CalldataBlobAcceptor } from "src/rollup/dataAvailability/CalldataBlobAcceptor.sol";

import { IPauseManager } from "src/security/pausing/interfaces/IPauseManager.sol";
import { IPermissionsManager } from "src/security/access/interfaces/IPermissionsManager.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import { IntegrationTestTrueVerifier } from "src/_testing/integration/IntegrationTestTrueVerifier.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";

/// @dev Mirrors TestLinethRollup: production LinethRollup plus calldata DA for unit tests.
contract LinethRollupTestHelper is LinethRollup, CalldataBlobAcceptor {
  function computeShnarf(
    bytes32 _parentShnarf,
    bytes32 _finalBlockHash,
    bytes32 _dataHash
  ) external pure returns (bytes32 shnarf) {
    return _computeShnarf(_parentShnarf, _finalBlockHash, _dataHash);
  }

  function computeDataRollingHash(bytes32 _parentDataRollingHash, bytes32 _chunkHash) external pure returns (bytes32) {
    return _computeDataRollingHash(_parentDataRollingHash, _chunkHash);
  }

  function computePositionCommitment(bytes32 _dataRollingHash, uint256 _offset) external pure returns (bytes32) {
    return _computePositionCommitment(_dataRollingHash, _offset);
  }

  function setupParentShnarf(bytes32 _dataRollingHash) external {
    _blobShnarfExists[_dataRollingHash] = 1;
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
    initData.shnarfProvider = address(0);
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

    assertEq(linethRollup.hasRole(DEFAULT_ADMIN_ROLE, defaultAdmin), true, "Default admin not set");
    assertEq(linethRollup.hasRole(OPERATOR_ROLE, operator), true, "Operator not set");
  }

  function securityCouncilSetup() internal view {
    // defaultAdmin doubles as security council in these unit tests.
    assertTrue(defaultAdmin != address(0));
  }

  function testSubmitDataAsCalldata() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";

    // Genesis DA stream position: parent dataRollingHash is the empty accumulator (offset 0).
    bytes32 parentDataRollingHash = bytes32(0);
    bytes32 chunkHash = keccak256(compressedData);
    bytes32 expectedDataRollingHash = linethRollup.computeDataRollingHash(parentDataRollingHash, chunkHash);

    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, parentDataRollingHash, expectedDataRollingHash);

    uint256 exists = linethRollup.blobShnarfExists(expectedDataRollingHash);
    assertEq(exists, 1, "Data rolling hash should be anchored after submission");
  }

  function testFinalizeBlocksHappyPath() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 finalBlockHash = keccak256("final-block-hash");

    // Genesis position: prev dataRollingHash = 0, prev offset = 0 (fresh start sentinel).
    bytes32 prevDataRollingHash = bytes32(0);
    uint256 prevOffset = 0;

    bytes32 chunkHash = keccak256(compressedData);
    bytes32 endDataRollingHash = linethRollup.computeDataRollingHash(prevDataRollingHash, chunkHash);

    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, prevDataRollingHash, endDataRollingHash);

    ILinethRollupBase.FinalizationDataV6 memory finalizationData;
    finalizationData.parentStateRootHash = bytes32(0);
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = finalBlockHash;
    finalizationData.prevDataRollingHash = prevDataRollingHash;
    finalizationData.prevOffset = prevOffset;
    finalizationData.parentDataRollingHash = prevDataRollingHash;
    finalizationData.endDataRollingHash = endDataRollingHash;
    finalizationData.startOffset = 0;
    finalizationData.endOffset = 100;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";

    vm.warp(10);
    vm.prank(operator);
    linethRollup.finalizeBlocks(hex"01", 0, finalizationData);

    bytes32 expectedPositionCommitment = linethRollup.computePositionCommitment(endDataRollingHash, 100);
    assertEq(linethRollup.blockHashes(10), finalBlockHash, "Final block hash not anchored");
    assertEq(linethRollup.currentL2BlockNumber(), 10, "Current L2 block not updated");
    assertEq(linethRollup.currentFinalizedShnarf(), expectedPositionCommitment, "Position commitment not updated");
  }

  function testComputeDataRollingHash() public view {
    bytes32 parent = keccak256("parent");
    bytes32 chunk = keccak256("chunk");
    bytes32 expected = keccak256(abi.encodePacked(parent, chunk));
    assertEq(linethRollup.computeDataRollingHash(parent, chunk), expected, "2-input dataRollingHash mismatch");
  }

  function testComputePositionCommitment() public view {
    bytes32 dataRollingHash = keccak256("drh");
    uint256 offset = 131072;
    bytes32 expected = keccak256(abi.encodePacked(dataRollingHash, bytes32(offset)));
    assertEq(linethRollup.computePositionCommitment(dataRollingHash, offset), expected, "Position commitment mismatch");
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

  function testSubmitDataAsCalldataRevertsOnWrongFinalHash() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 parentDataRollingHash = bytes32(0);
    bytes32 wrongFinal = keccak256("wrong");
    // Precompute before the prank so the helper call is not intercepted by expectRevert.
    bytes32 computedFinal = linethRollup.computeDataRollingHash(parentDataRollingHash, keccak256(compressedData));

    vm.expectRevert(
      abi.encodeWithSelector(bytes4(keccak256("FinalDataRollingHashWrong(bytes32,bytes32)")), wrongFinal, computedFinal)
    );
    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, parentDataRollingHash, wrongFinal);
  }

  function testFinalizeBlocksRevertsOnUnanchoredEndDataRollingHash() public {
    bytes32 prevDataRollingHash = bytes32(0);

    ILinethRollupBase.FinalizationDataV6 memory finalizationData;
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = keccak256("final-block-hash");
    finalizationData.prevDataRollingHash = prevDataRollingHash;
    finalizationData.prevOffset = 0;
    finalizationData.parentDataRollingHash = prevDataRollingHash;
    finalizationData.endDataRollingHash = keccak256("never-anchored");
    finalizationData.startOffset = 0;
    finalizationData.endOffset = 100;
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

  function testFinalizeBlocksRevertsOnPositionCommitmentMismatch() public {
    bytes memory compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";
    bytes32 endDataRollingHash = linethRollup.computeDataRollingHash(bytes32(0), keccak256(compressedData));

    vm.prank(operator);
    linethRollup.submitDataAsCalldata(compressedData, bytes32(0), endDataRollingHash);

    ILinethRollupBase.FinalizationDataV6 memory finalizationData;
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = keccak256("final-block-hash");
    // Wrong previous position: does not open the stored genesis commitment.
    finalizationData.prevDataRollingHash = keccak256("wrong-prev");
    finalizationData.prevOffset = 0;
    finalizationData.parentDataRollingHash = keccak256("wrong-prev");
    finalizationData.endDataRollingHash = endDataRollingHash;
    finalizationData.startOffset = 0;
    finalizationData.endOffset = 100;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";

    vm.warp(10);
    vm.prank(operator);
    vm.expectRevert(); // PositionCommitmentMismatch
    linethRollup.finalizeBlocks(hex"01", 0, finalizationData);
  }

  function testReinitializeLineaRollupV11BridgeRevertsOnMismatch() public {
    // The live proxy's position-commitment slot holds the genesis commitment, so bridging against a
    // different value must revert with BridgedShnarfMismatch.
    bytes32 wrongLegacyShnarf = keccak256("legacy-finalized-shnarf");
    vm.expectRevert(); // BridgedShnarfMismatch
    linethRollup.reinitializeLineaRollupV11(wrongLegacyShnarf);
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
