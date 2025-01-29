// SPDX-License-Identifier: Apache-2.0
pragma solidity >=0.8.19 <=0.8.26;

/**
 * @title Interface declaring generic errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IGenericErrors {
  /**
   * @dev Thrown when a parameter is the zero address.
   */
  error ZeroAddressNotAllowed();

  /**
   * @dev Thrown when a parameter is the zero hash.
   */
  error ZeroHashNotAllowed();

  /**
   * @dev Thrown when array lengths are mismatched.
   */
  error ArrayLengthsDoNotMatch();
}
