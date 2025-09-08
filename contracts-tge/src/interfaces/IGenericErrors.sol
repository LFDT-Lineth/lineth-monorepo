// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.30;

/**
 * @title Interface declaring generic errors.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IGenericErrors {
  /**
   * @dev Thrown when a parameter is the zero address.
   */
  error ZeroAddressNotAllowed();
}
