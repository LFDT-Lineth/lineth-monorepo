// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title L2 Message Service interface for pre-existing functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IL2MessageServiceV1 {
  /**
   * @notice Emitted when the L2 message service is initialized.
   * @param contractVersion The contract version.
   */
  event L2MessageServiceBaseInitialized(bytes8 indexed contractVersion);

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() external view returns (string memory contractVersion);

  /**
   * @notice The Fee Manager sets a minimum fee to address DOS protection.
   * @dev MINIMUM_FEE_SETTER_ROLE is required to set the minimum fee.
   * @param _feeInWei New minimum fee in Wei.
   */
  function setMinimumFee(uint256 _feeInWei) external;
}
