// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.26;

/**
 * @title Library that provides helper functions to interact with transient storage.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
library TransientStorageHelpers {
  /**
   * @notice Internal function that stores a uint256 value at a given key in the EVM's transient storage using the `tstore` opcode.
   * @param _key The key in the EVM transient storage where the value should be stored.
   * @param _value The uint256 value to be stored at the specified key in the EVM transient storage.
   */
  function tstoreUint256(bytes32 _key, uint256 _value) internal {
    assembly {
      tstore(_key, _value)
    }
  }

  /**
   * @notice Internal function that retrieves a uint256 value from the EVM's transient storage using the `tload` opcode.
   * @param _key The key in the EVM transient storage from which the value should be retrieved.
   * @return value The uint256 value retrieved from the specified key in the EVM transient storage.
   */
  function tloadUint256(bytes32 _key) internal view returns (uint256 value) {
    assembly {
      value := tload(_key)
    }
  }

  /**
   * @notice Internal function that stores an address at a given key in the EVM's transient storage using the `tstore` opcode.
   * @param _key The key in the EVM transient storage where the value should be stored.
   * @param _addr The address to be stored at the specified key in the EVM transient storage.
   */
  function tstoreAddress(bytes32 _key, address _addr) internal {
    assembly {
      tstore(_key, _addr)
    }
  }

  /**
   * @notice Internal function that retrieves an address from the EVM's transient storage using the `tload` opcode.
   * @param _key The key in the EVM transient storage from which the value should be retrieved.
   * @return addr The address retrieved from the specified key in the EVM transient storage.
   */
  function tloadAddress(bytes32 _key) internal view returns (address addr) {
    assembly {
      addr := tload(_key)
    }
  }
}
