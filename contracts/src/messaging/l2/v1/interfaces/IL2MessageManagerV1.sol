// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

/**
 * @title Interface declaring pre-existing cross-chain messaging on L2 functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IL2MessageManagerV1 {
  /**
   * @notice Emitted when L2 minimum fee is changed.
   * @param previousMinimumFee The previous minimum fee in Wei.
   * @param newMinimumFee The new minimum fee in Wei.
   * @param calledBy The indexed address who changed the minimum fee.
   */
  event MinimumFeeChanged(uint256 previousMinimumFee, uint256 newMinimumFee, address indexed calledBy);

  /**
   * @notice Emitted when L1->L2 message hashes have been added to L2 storage.
   * @param messageHashes The message hashes that were added to L2 for claiming.
   */
  event L1L2MessageHashesAddedToInbox(bytes32[] messageHashes);

  /**
   * @dev Thrown when the message hashes list length is higher than one hundred.
   */
  error MessageHashesListLengthHigherThanOneHundred(uint256 length);

  /**
   * @dev Thrown when the message does not exist or has already been claimed.
   */
  error MessageDoesNotExistOrHasAlreadyBeenClaimed(bytes32 messageHash);
}
