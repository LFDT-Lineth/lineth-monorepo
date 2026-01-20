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
   * @dev Thrown when no ETH is sent.
   */
  error NoEthSent();

  /**
   * @dev Thrown when the caller is not the ProxyAdmin.
   */
  error CallerNotProxyAdmin();
}
