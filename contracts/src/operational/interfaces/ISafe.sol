// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.33;

/**
 * @title Minimal interface for a multisig Safe.
 * @notice Exposes only the subset of the Safe API consumed by operational helpers.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ISafe {
  /**
   * @notice Returns whether an address is an owner of the Safe.
   * @param _owner The address to check ownership for.
   * @return isAnOwner True when `_owner` is registered as a Safe owner.
   */
  function isOwner(address _owner) external view returns (bool isAnOwner);
}
