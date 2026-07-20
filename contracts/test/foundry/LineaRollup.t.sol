// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import "forge-std/Test.sol";

import { LineaRollup } from "src/rollup/LineaRollup.sol";
import { ILineaRollupBase } from "src/rollup/interfaces/ILineaRollupBase.sol";
import { IAcceptCalldataBlobs } from "src/rollup/dataAvailability/interfaces/IAcceptCalldataBlobs.sol";
import { CalldataBlobAcceptor } from "src/rollup/dataAvailability/CalldataBlobAcceptor.sol";

import { IPauseManager } from "src/security/pausing/interfaces/IPauseManager.sol";
import { IPermissionsManager } from "src/security/access/interfaces/IPermissionsManager.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import { IntegrationTestTrueVerifier } from "src/_testing/integration/IntegrationTestTrueVerifier.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";

/// @dev Mirrors TestLineaRollup: production LineaRollup plus calldata DA for unit tests.
contract LineaRollupTestHelper is LineaRollup, CalldataBlobAcceptor {
  function computeShnarf(
    bytes32 _parentShnarf,
    bytes32 _finalBlockHash,
    bytes32 _dataHash
  ) external pure returns (bytes32 shnarf) {
    return _computeShnarf(_parentShnarf, _finalBlockHash, _dataHash);
  }

  function setupParentShnarf(bytes32 _shnarf) external {
    _blobShnarfExists[_shnarf] = 1;
  }

  function renounceRole(
    bytes32 _role,
    address _account
  ) public virtual override(LineaRollup, AccessControlUpgradeable) {
    super.renounceRole(_role, _account);
  }
}

contract LineaRollupTest is Test {
  LineaRollupTestHelper lineaRollup;
  LineaRollupTestHelper implementation;
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

    implementation = new LineaRollupTestHelper();

    ILineaRollupBase.BaseInitializationData memory initData;
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
      LineaRollup.initialize.selector,
      initData,
      fallbackOperator,
      yieldManager
    );

    ERC1967Proxy proxy = new ERC1967Proxy(address(implementation), initializer);

    lineaRollup = LineaRollupTestHelper(address(proxy));

    VERIFIER_SETTER_ROLE = lineaRollup.VERIFIER_SETTER_ROLE();
    VERIFIER_UNSETTER_ROLE = lineaRollup.VERIFIER_UNSETTER_ROLE();
    OPERATOR_ROLE = lineaRollup.OPERATOR_ROLE();
    DEFAULT_ADMIN_ROLE = lineaRollup.DEFAULT_ADMIN_ROLE();

    assertEq(lineaRollup.hasRole(DEFAULT_ADMIN_ROLE, defaultAdmin), true, "Default admin not set");
    assertEq(lineaRollup.hasRole(OPERATOR_ROLE, operator), true, "Operator not set");
  }

  function securityCouncilSetup() internal view {
    // defaultAdmin doubles as security council in these unit tests.
    assertTrue(defaultAdmin != address(0));
  }

  function testSubmitDataAsCalldata() public {
    IAcceptCalldataBlobs.CompressedCalldataSubmissionV2 memory submission;
    submission.blockHash = keccak256("final-block-hash");
    submission.compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";

    bytes32 parentShnarf = lineaRollup.computeShnarf(bytes32(0), INITIAL_BLOCK_HASH, bytes32(0));
    bytes32 dataHash = keccak256(submission.compressedData);
    bytes32 expectedShnarf = lineaRollup.computeShnarf(parentShnarf, submission.blockHash, dataHash);

    vm.prank(operator);
    lineaRollup.submitDataAsCalldata(submission, parentShnarf, expectedShnarf);

    uint256 exists = lineaRollup.blobShnarfExists(expectedShnarf);
    assertEq(exists, 1, "Blob shnarf should exist after submission");
  }

  function testFinalizeBlocksHappyPath() public {
    IAcceptCalldataBlobs.CompressedCalldataSubmissionV2 memory submission;
    submission.blockHash = keccak256("final-block-hash");
    submission.compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";

    bytes32 parentShnarf = lineaRollup.computeShnarf(bytes32(0), INITIAL_BLOCK_HASH, bytes32(0));
    bytes32 dataHash = keccak256(submission.compressedData);
    bytes32 expectedShnarf = lineaRollup.computeShnarf(parentShnarf, submission.blockHash, dataHash);

    vm.prank(operator);
    lineaRollup.submitDataAsCalldata(submission, parentShnarf, expectedShnarf);

    ILineaRollupBase.FinalizationDataV5 memory finalizationData;
    finalizationData.parentStateRootHash = bytes32(0);
    finalizationData.parentBlockHash = INITIAL_BLOCK_HASH;
    finalizationData.endBlockNumber = 10;
    finalizationData.shnarfData.parentShnarf = parentShnarf;
    finalizationData.lastFinalizedTimestamp = 1;
    finalizationData.finalTimestamp = 2;
    finalizationData.finalBlockHash = submission.blockHash;
    finalizationData.finalBlobHash = dataHash;
    finalizationData.l2MerkleRoots = new bytes32[](0);
    finalizationData.filteredAddresses = new address[](0);
    finalizationData.verifierKeys = new bytes32[](0);
    finalizationData.l2MessagingBlocksOffsets = hex"";

    vm.warp(10);
    vm.prank(operator);
    lineaRollup.finalizeBlocks(hex"01", 0, finalizationData);

    assertEq(lineaRollup.blockHashes(10), submission.blockHash, "Final block hash not anchored");
    assertEq(lineaRollup.currentL2BlockNumber(), 10, "Current L2 block not updated");
    assertEq(lineaRollup.currentFinalizedShnarf(), expectedShnarf, "Finalized shnarf not updated");
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
    lineaRollup.setVerifierAddress(newVerifier, 2);
  }

  function testSetVerifierAddressSuccess() public {
    vm.startPrank(defaultAdmin);
    lineaRollup.grantRole(VERIFIER_SETTER_ROLE, defaultAdmin);
    vm.stopPrank();

    address newVerifier = address(0x1234);

    vm.prank(defaultAdmin);
    lineaRollup.setVerifierAddress(newVerifier, 2);

    assertEq(lineaRollup.verifiers(2), newVerifier, "Verifier address not updated");
  }

  function testUnsetVerifierAddress() public {
    vm.startPrank(defaultAdmin);
    lineaRollup.grantRole(VERIFIER_UNSETTER_ROLE, defaultAdmin);
    lineaRollup.grantRole(VERIFIER_SETTER_ROLE, defaultAdmin);

    address newVerifier = address(0x1234);
    lineaRollup.setVerifierAddress(newVerifier, 0);
    vm.stopPrank();

    vm.prank(defaultAdmin);
    lineaRollup.unsetVerifierAddress(0);

    assertEq(lineaRollup.verifiers(0), address(0), "Verifier address not unset");
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
    lineaRollup.unsetVerifierAddress(0);
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
