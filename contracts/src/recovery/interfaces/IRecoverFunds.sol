// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.26;

/**
 * @title Interface declaring IRecoverFunds errors and functions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IRecoverFunds {
  /**
   * @dev Thrown when the external call failed.
   * @param destinationAddress the destination address called.
   */
  error ExternalCallFailed(address destinationAddress);

  /**
   * @notice Executes external calls.
   * @param _destination The address being called.
   * @param _callData The calldata being sent to the address.
   * @param _ethValue Any ETH value being sent.
   * @dev "0x" for calldata can be used for simple ETH transfers.
   */
  function executeExternalCall(address _destination, bytes memory _callData, uint256 _ethValue) external payable;
}
