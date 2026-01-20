// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IGenericErrors } from "../interfaces/IGenericErrors.sol";

/**
 * @title Library for error checking utilities.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
library ErrorUtils {
  /**
   * @notice Reverts if the address is the zero address.
   * @param _addr The address to check.
   */
  function revertIfZeroAddress(address _addr) internal pure {
    if (_addr == address(0)) revert IGenericErrors.ZeroAddressNotAllowed();
  }

  /**
   * @notice Reverts if the hash is the zero hash.
   * @param _hash The hash to check.
   */
  function revertIfZeroHash(bytes32 _hash) internal pure {
    if (_hash == bytes32(0)) revert IGenericErrors.ZeroHashNotAllowed();
  }
}
