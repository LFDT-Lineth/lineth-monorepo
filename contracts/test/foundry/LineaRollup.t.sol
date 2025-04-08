// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.28;

import "forge-std/Test.sol";

import { EfficientLeftRightKeccak } from "src/libraries/EfficientLeftRightKeccak.sol";
import { LineaRollup } from "src/rollup/LineaRollup.sol";
import { ILineaRollup } from "src/rollup/interfaces/ILineaRollup.sol";

import { IPauseManager } from "src/security/pausing/interfaces/IPauseManager.sol";
import { IPermissionsManager } from "src/security/access/interfaces/IPermissionsManager.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

contract LineaRollupTestHelper is LineaRollup {
  function calculateY(bytes calldata data, bytes32 dataEvaluationPoint) external pure returns (bytes32) {
    return _calculateY(data, dataEvaluationPoint);
  }

  function computeShnarf(
    bytes32 _parentShnarf,
    bytes32 _snarkHash,
    bytes32 _finalStateRootHash,
    bytes32 _dataEvaluationPoint,
    bytes32 _dataEvaluationClaim
  ) external pure returns (bytes32 shnarf) {
    return _computeShnarf(_parentShnarf, _snarkHash, _finalStateRootHash, _dataEvaluationPoint, _dataEvaluationClaim);
  }
}

contract LineaRollupTest is Test {
  LineaRollupTestHelper lineaRollup;
  LineaRollupTestHelper implementation;
  address operator;
  address defaultAdmin;
  address verifier;
  address nonAuthorizedAccount;
  address securityCouncil;
  address fallbackOperator;

  bytes32 VERIFIER_SETTER_ROLE;
  bytes32 VERIFIER_UNSETTER_ROLE;
  bytes32 OPERATOR_ROLE;
  bytes32 DEFAULT_ADMIN_ROLE;

  function setUp() public {
    operator = address(0x1);
    defaultAdmin = address(0x2);
    verifier = address(0x3);
    securityCouncil = defaultAdmin;
    fallbackOperator = address(0x4);
    nonAuthorizedAccount = address(0x5);

    implementation = new LineaRollupTestHelper();

    ILineaRollup.InitializationData memory initData;
    initData.initialStateRootHash = bytes32(0x0);
    initData.initialL2BlockNumber = 0;
    initData.genesisTimestamp = block.timestamp;
    initData.defaultVerifier = verifier;
    initData.rateLimitPeriodInSeconds = 86400; // 1 day
    initData.rateLimitAmountInWei = 100 ether;

    initData.roleAddresses = new IPermissionsManager.RoleAddress[](1);
    initData.roleAddresses[0] = IPermissionsManager.RoleAddress({
      addressWithRole: operator,
      role: implementation.OPERATOR_ROLE()
    });

    initData.pauseTypeRoles = new IPauseManager.PauseTypeRole[](0);
    initData.unpauseTypeRoles = new IPauseManager.PauseTypeRole[](0);
    initData.fallbackOperator = fallbackOperator;
    initData.defaultAdmin = defaultAdmin;

    bytes memory initializer = abi.encodeWithSelector(LineaRollup.initialize.selector, initData);

    ERC1967Proxy proxy = new ERC1967Proxy(address(implementation), initializer);

    lineaRollup = LineaRollupTestHelper(address(proxy));

    VERIFIER_SETTER_ROLE = lineaRollup.VERIFIER_SETTER_ROLE();
    VERIFIER_UNSETTER_ROLE = lineaRollup.VERIFIER_UNSETTER_ROLE();
    OPERATOR_ROLE = lineaRollup.OPERATOR_ROLE();
    DEFAULT_ADMIN_ROLE = lineaRollup.DEFAULT_ADMIN_ROLE();

    assertEq(lineaRollup.hasRole(DEFAULT_ADMIN_ROLE, defaultAdmin), true, "Default admin not set");
    assertEq(lineaRollup.hasRole(OPERATOR_ROLE, operator), true, "Operator not set");
  }

  function testSubmitDataAsCalldata() public {
    ILineaRollup.CompressedCalldataSubmission memory submission;
    submission.finalStateRootHash = keccak256(abi.encodePacked("finalStateRootHash"));
    submission.snarkHash = keccak256(abi.encodePacked("snarkHash"));

    // Adjust compressedData to start with 0x00
    submission.compressedData = hex"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff";

    bytes32 dataEvaluationPoint = EfficientLeftRightKeccak._efficientKeccak(
      submission.snarkHash,
      keccak256(submission.compressedData)
    );

    bytes32 dataEvaluationClaim = lineaRollup.calculateY(submission.compressedData, dataEvaluationPoint);

    bytes32 parentShnarf = lineaRollup.computeShnarf(0x0, 0x0, 0x0, 0x0, 0x0);

    bytes32 expectedShnarf = keccak256(
      abi.encodePacked(
        parentShnarf,
        submission.snarkHash,
        submission.finalStateRootHash,
        dataEvaluationPoint,
        dataEvaluationClaim
      )
    );

    vm.prank(operator);
    lineaRollup.submitDataAsCalldata(submission, parentShnarf, expectedShnarf);

    uint256 exists = lineaRollup.blobShnarfExists(expectedShnarf);
    assertEq(exists, 1, "Blob shnarf should exist after submission");
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

  // Helper function to convert address to ascii string
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

  // Helper function to convert byte to char
  function _char(uint8 b) internal pure returns (bytes1 c) {
    if (b < 10) {
      return bytes1(b + 0x30);
    } else {
      return bytes1(b + 0x57);
    }
  }

  // Helper function to convert bytes32 to hex string
  function _toHexString(bytes32 data) internal pure returns (string memory) {
    return _toHexString(abi.encodePacked(data));
  }

  // Helper function to convert bytes to hex string
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
