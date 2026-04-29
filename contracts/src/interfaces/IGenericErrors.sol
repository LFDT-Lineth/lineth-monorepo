// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

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
   * @dev Thrown when a parameter is zero.
   */
  error ZeroValueNotAllowed();

  /**
   * @dev Thrown when no ETH is sent.
   */
  error NoEthSent();

  /**
   * @dev Thrown when an initialize function is called on an already initialized contract with the wrong version.
   * @param expected The expected initialized version.
   * @param actual The actual initialized version.
   */
  error InitializedVersionWrong(uint256 expected, uint256 actual);

  /**
   * @dev Thrown when a parameter is the zero length.
   */
  error ZeroLengthNotAllowed();
}
