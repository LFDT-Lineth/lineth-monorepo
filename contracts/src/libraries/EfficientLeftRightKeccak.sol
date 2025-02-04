// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

/**
 * @title Contract to manage some efficient hashing functions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
library EfficientLeftRightKeccak {
  /**
   * @notice Performs a gas optimized keccak hash for two bytes32 values.
   * @param _left Left value.
   * @param _right Right value.
   */
  function _efficientKeccak(bytes32 _left, bytes32 _right) internal pure returns (bytes32 value) {
    /// @solidity memory-safe-assembly
    assembly {
      mstore(0x00, _left)
      mstore(0x20, _right)
      value := keccak256(0x00, 0x40)
    }
  }

  /**
   * @notice Performs a gas optimized keccak hash for uint256 and address.
   * @param _left Left value.
   * @param _right Right value.
   */
  function _efficientKeccak(uint256 _left, address _right) internal pure returns (bytes32 value) {
    /// @solidity memory-safe-assembly
    assembly {
      mstore(0x00, _left)
      mstore(0x20, _right)
      value := keccak256(0x00, 0x40)
    }
  }
}
